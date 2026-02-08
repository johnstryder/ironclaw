package tooling

import (
	"bytes"
	"context"
	"io"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// dockerAPIClient is the subset of Docker Engine API methods used by
// DockerContainerRuntime. Defined as an interface so tests can inject a mock
// instead of talking to a real Docker daemon.
type dockerAPIClient interface {
	ImagePull(ctx context.Context, refStr string, options client.ImagePullOptions) (imagePullResponse, error)
	ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error)
	ContainerWait(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult
	ContainerLogs(ctx context.Context, containerID string, options client.ContainerLogsOptions) (client.ContainerLogsResult, error)
	ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	Close() error
}

// imagePullResponse is the subset of client.ImagePullResponse we use.
// We define our own so the mock doesn't need to import iter/jsonstream.
// The real client.ImagePullResponse satisfies this (it embeds io.ReadCloser).
type imagePullResponse interface {
	io.ReadCloser
}

// dockerClientAdapter wraps *client.Client to satisfy dockerAPIClient.
// The adapter narrows ImagePull's return type from client.ImagePullResponse
// (which includes JSONMessages/Wait) down to imagePullResponse (io.ReadCloser),
// since EnsureImage only needs to drain and close the reader.
type dockerClientAdapter struct {
	cli *client.Client
}

// Compile-time proof that the adapter satisfies our interface.
var _ dockerAPIClient = (*dockerClientAdapter)(nil)

func (a *dockerClientAdapter) ImagePull(ctx context.Context, ref string, opts client.ImagePullOptions) (imagePullResponse, error) {
	return a.cli.ImagePull(ctx, ref, opts)
}
func (a *dockerClientAdapter) ContainerCreate(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	return a.cli.ContainerCreate(ctx, opts)
}
func (a *dockerClientAdapter) ContainerStart(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error) {
	return a.cli.ContainerStart(ctx, id, opts)
}
func (a *dockerClientAdapter) ContainerWait(ctx context.Context, id string, opts client.ContainerWaitOptions) client.ContainerWaitResult {
	return a.cli.ContainerWait(ctx, id, opts)
}
func (a *dockerClientAdapter) ContainerLogs(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	return a.cli.ContainerLogs(ctx, id, opts)
}
func (a *dockerClientAdapter) ContainerRemove(ctx context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	return a.cli.ContainerRemove(ctx, id, opts)
}
func (a *dockerClientAdapter) Close() error {
	return a.cli.Close()
}

// DockerContainerRuntime implements ContainerRuntime using the Docker Engine API
// via the moby/moby/client SDK.
type DockerContainerRuntime struct {
	api dockerAPIClient
}

// newDockerClientFunc creates the Docker API client.
// Package-level so tests can inject a failing factory to cover the error path.
var newDockerClientFunc = func() (dockerAPIClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &dockerClientAdapter{cli: cli}, nil
}

// NewDockerContainerRuntime creates a new DockerContainerRuntime connected to
// the local Docker daemon using environment defaults (DOCKER_HOST, etc.).
func NewDockerContainerRuntime() (*DockerContainerRuntime, error) {
	api, err := newDockerClientFunc()
	if err != nil {
		return nil, err
	}
	return &DockerContainerRuntime{api: api}, nil
}

// EnsureImage pulls the specified Docker image if it is not already available locally.
func (d *DockerContainerRuntime) EnsureImage(ctx context.Context, ref string) error {
	resp, err := d.api.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()
	// Drain the reader to complete the pull (progress output is discarded)
	_, err = io.Copy(io.Discard, resp)
	return err
}

// CreateContainer creates a new container with the given sandbox configuration.
// The container is configured with hardened isolation:
//   - Network disabled (NetworkDisabled in Config + NetworkMode "none" in HostConfig)
//   - Memory limit enforced (MemorySwap = Memory to prevent swap)
//   - CPU limit enforced (NanoCPUs)
//   - PID limit enforced (prevents fork bombs)
//   - No new privileges (security_opt: no-new-privileges)
//   - All capabilities dropped
//   - Readonly root filesystem with tmpfs for /tmp
//   - No host volume binds
//   - Unprivileged mode
func (d *DockerContainerRuntime) CreateContainer(ctx context.Context, cfg SandboxContainerConfig) (string, error) {
	pidsLimit := cfg.PidsLimit
	opts := client.ContainerCreateOptions{
		Config: &container.Config{
			Image:           cfg.Image,
			Cmd:             cfg.Cmd,
			NetworkDisabled: cfg.NetworkDisabled,
		},
		HostConfig: &container.HostConfig{
			Resources: container.Resources{
				Memory:     cfg.MemoryLimit,
				MemorySwap: cfg.MemoryLimit, // Prevent swap usage beyond memory limit
				NanoCPUs:   cfg.CPULimit,     // CPU limit to prevent DoS
				PidsLimit:  &pidsLimit,       // Prevent fork bombs
			},
			NetworkMode:    "none",  // Defense-in-depth: block network at host level
			ReadonlyRootfs: true,    // Prevent writes to root filesystem
			Tmpfs:          map[string]string{"/tmp": "size=16m,noexec,nosuid"}, // Writable /tmp with size limit
			SecurityOpt:    []string{"no-new-privileges"},
			CapDrop:        []string{"ALL"},
			Privileged:     false, // Explicitly ensure unprivileged
			AutoRemove:     false, // We manage removal via defer
		},
	}

	resp, err := d.api.ContainerCreate(ctx, opts)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// StartContainer starts a previously created container.
func (d *DockerContainerRuntime) StartContainer(ctx context.Context, containerID string) error {
	_, err := d.api.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	return err
}

// WaitContainer blocks until the container exits and returns the exit code.
func (d *DockerContainerRuntime) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	result := d.api.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	select {
	case err := <-result.Error:
		if err != nil {
			return -1, err
		}
		return -1, nil
	case status := <-result.Result:
		return status.StatusCode, nil
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

// GetLogs retrieves the combined stdout and stderr logs from the container.
func (d *DockerContainerRuntime) GetLogs(ctx context.Context, containerID string) (string, error) {
	reader, err := d.api.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RemoveContainer forcefully removes a container and its volumes.
func (d *DockerContainerRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	_, err := d.api.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	return err
}

// Close closes the Docker client connection.
func (d *DockerContainerRuntime) Close() error {
	return d.api.Close()
}
