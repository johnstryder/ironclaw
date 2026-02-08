package tooling

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// failTransport is an http.RoundTripper that always returns an error.
// Used to exercise the adapter without a real Docker daemon.
type failTransport struct{}

func (f *failTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, errors.New("no docker daemon")
}

// =============================================================================
// Test Doubles for dockerAPIClient
// =============================================================================

// mockDockerAPI implements dockerAPIClient with configurable function fields.
type mockDockerAPI struct {
	imagePullFn       func(ctx context.Context, ref string, opts client.ImagePullOptions) (imagePullResponse, error)
	containerCreateFn func(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	containerStartFn  func(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error)
	containerWaitFn   func(ctx context.Context, id string, opts client.ContainerWaitOptions) client.ContainerWaitResult
	containerLogsFn   func(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error)
	containerRemoveFn func(ctx context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	closeFn           func() error
}

func (m *mockDockerAPI) ImagePull(ctx context.Context, ref string, opts client.ImagePullOptions) (imagePullResponse, error) {
	return m.imagePullFn(ctx, ref, opts)
}
func (m *mockDockerAPI) ContainerCreate(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	return m.containerCreateFn(ctx, opts)
}
func (m *mockDockerAPI) ContainerStart(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error) {
	return m.containerStartFn(ctx, id, opts)
}
func (m *mockDockerAPI) ContainerWait(ctx context.Context, id string, opts client.ContainerWaitOptions) client.ContainerWaitResult {
	return m.containerWaitFn(ctx, id, opts)
}
func (m *mockDockerAPI) ContainerLogs(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	return m.containerLogsFn(ctx, id, opts)
}
func (m *mockDockerAPI) ContainerRemove(ctx context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	return m.containerRemoveFn(ctx, id, opts)
}
func (m *mockDockerAPI) Close() error {
	return m.closeFn()
}

// errReader is an io.ReadCloser that always returns an error on Read.
type errReader struct{ err error }

func (e *errReader) Read(_ []byte) (int, error) { return 0, e.err }
func (e *errReader) Close() error               { return nil }

// happyDockerAPI returns a mockDockerAPI that succeeds at every operation.
func happyDockerAPI() *mockDockerAPI {
	return &mockDockerAPI{
		imagePullFn: func(_ context.Context, _ string, _ client.ImagePullOptions) (imagePullResponse, error) {
			return io.NopCloser(strings.NewReader("")), nil
		},
		containerCreateFn: func(_ context.Context, _ client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
			return client.ContainerCreateResult{ID: "happy-container"}, nil
		},
		containerStartFn: func(_ context.Context, _ string, _ client.ContainerStartOptions) (client.ContainerStartResult, error) {
			return client.ContainerStartResult{}, nil
		},
		containerWaitFn: func(_ context.Context, _ string, _ client.ContainerWaitOptions) client.ContainerWaitResult {
			statusCh := make(chan container.WaitResponse, 1)
			errCh := make(chan error, 1)
			statusCh <- container.WaitResponse{StatusCode: 0}
			return client.ContainerWaitResult{Result: statusCh, Error: errCh}
		},
		containerLogsFn: func(_ context.Context, _ string, _ client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
			return io.NopCloser(strings.NewReader("logs output")), nil
		},
		containerRemoveFn: func(_ context.Context, _ string, _ client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
			return client.ContainerRemoveResult{}, nil
		},
		closeFn: func() error { return nil },
	}
}

// =============================================================================
// NewDockerContainerRuntime
// =============================================================================

func TestNewDockerContainerRuntime_ShouldReturnRuntimeWhenClientCreationSucceeds(t *testing.T) {
	original := newDockerClientFunc
	newDockerClientFunc = func() (dockerAPIClient, error) {
		return happyDockerAPI(), nil
	}
	defer func() { newDockerClientFunc = original }()

	rt, err := NewDockerContainerRuntime()
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if rt == nil {
		t.Fatal("Expected non-nil runtime")
	}
}

func TestNewDockerContainerRuntime_ShouldReturnErrorWhenClientCreationFails(t *testing.T) {
	original := newDockerClientFunc
	newDockerClientFunc = func() (dockerAPIClient, error) {
		return nil, fmt.Errorf("connection refused")
	}
	defer func() { newDockerClientFunc = original }()

	rt, err := NewDockerContainerRuntime()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if rt != nil {
		t.Error("Expected nil runtime on error")
	}
}

// =============================================================================
// EnsureImage
// =============================================================================

func TestEnsureImage_ShouldSucceedWhenPullAndDrainSucceed(t *testing.T) {
	api := happyDockerAPI()
	rt := &DockerContainerRuntime{api: api}
	err := rt.EnsureImage(context.Background(), "alpine:latest")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
}

func TestEnsureImage_ShouldReturnErrorWhenPullFails(t *testing.T) {
	api := happyDockerAPI()
	api.imagePullFn = func(_ context.Context, _ string, _ client.ImagePullOptions) (imagePullResponse, error) {
		return nil, errors.New("pull failed")
	}
	rt := &DockerContainerRuntime{api: api}
	err := rt.EnsureImage(context.Background(), "missing:image")
	if err == nil {
		t.Fatal("Expected error when pull fails")
	}
	if !strings.Contains(err.Error(), "pull failed") {
		t.Errorf("Expected 'pull failed', got: %v", err)
	}
}

func TestEnsureImage_ShouldReturnErrorWhenDrainFails(t *testing.T) {
	api := happyDockerAPI()
	api.imagePullFn = func(_ context.Context, _ string, _ client.ImagePullOptions) (imagePullResponse, error) {
		return &errReader{err: errors.New("drain error")}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	err := rt.EnsureImage(context.Background(), "alpine:latest")
	if err == nil {
		t.Fatal("Expected error when drain fails")
	}
	if !strings.Contains(err.Error(), "drain error") {
		t.Errorf("Expected 'drain error', got: %v", err)
	}
}

// =============================================================================
// CreateContainer
// =============================================================================

func TestCreateContainer_ShouldReturnContainerIDOnSuccess(t *testing.T) {
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, _ client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		return client.ContainerCreateResult{ID: "abc-123"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	id, err := rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "alpine:latest",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		NetworkDisabled: true,
	})
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if id != "abc-123" {
		t.Errorf("Expected 'abc-123', got '%s'", id)
	}
}

func TestCreateContainer_ShouldReturnErrorWhenCreateFails(t *testing.T) {
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, _ client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		return client.ContainerCreateResult{}, errors.New("no space left")
	}
	rt := &DockerContainerRuntime{api: api}
	id, err := rt.CreateContainer(context.Background(), SandboxContainerConfig{})
	if err == nil {
		t.Fatal("Expected error when create fails")
	}
	if id != "" {
		t.Errorf("Expected empty ID on error, got '%s'", id)
	}
}

func TestCreateContainer_ShouldPassSecurityConfigToDocker(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"python3", "-c", "print(1)"},
		MemoryLimit:     67108864,
		NetworkDisabled: true,
	})
	if capturedOpts.Config.NetworkDisabled != true {
		t.Error("Expected NetworkDisabled=true")
	}
	if capturedOpts.HostConfig.Memory != 67108864 {
		t.Errorf("Expected memory=67108864, got %d", capturedOpts.HostConfig.Memory)
	}
	if len(capturedOpts.HostConfig.SecurityOpt) == 0 || capturedOpts.HostConfig.SecurityOpt[0] != "no-new-privileges" {
		t.Error("Expected SecurityOpt to contain 'no-new-privileges'")
	}
	if len(capturedOpts.HostConfig.CapDrop) == 0 || capturedOpts.HostConfig.CapDrop[0] != "ALL" {
		t.Error("Expected CapDrop to contain 'ALL'")
	}
}

func TestCreateContainer_ShouldSetNetworkModeToNone(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		NetworkDisabled: true,
	})
	if capturedOpts.HostConfig.NetworkMode != "none" {
		t.Errorf("Expected NetworkMode='none', got '%s'", capturedOpts.HostConfig.NetworkMode)
	}
}

func TestCreateContainer_ShouldSetCPULimitAsNanoCPUs(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		CPULimit:        500_000_000,
		NetworkDisabled: true,
	})
	if capturedOpts.HostConfig.NanoCPUs != 500_000_000 {
		t.Errorf("Expected NanoCPUs=500000000, got %d", capturedOpts.HostConfig.NanoCPUs)
	}
}

func TestCreateContainer_ShouldSetPidsLimit(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		PidsLimit:       64,
		NetworkDisabled: true,
	})
	if capturedOpts.HostConfig.PidsLimit == nil {
		t.Fatal("Expected PidsLimit to be set, got nil")
	}
	if *capturedOpts.HostConfig.PidsLimit != 64 {
		t.Errorf("Expected PidsLimit=64, got %d", *capturedOpts.HostConfig.PidsLimit)
	}
}

func TestCreateContainer_ShouldSetReadonlyRootfs(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		NetworkDisabled: true,
	})
	if !capturedOpts.HostConfig.ReadonlyRootfs {
		t.Error("Expected ReadonlyRootfs=true for hardened isolation")
	}
}

func TestCreateContainer_ShouldSetTmpfsForTmp(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		NetworkDisabled: true,
	})
	tmpfs := capturedOpts.HostConfig.Tmpfs
	if tmpfs == nil {
		t.Fatal("Expected Tmpfs to be set for /tmp writable directory")
	}
	opts, exists := tmpfs["/tmp"]
	if !exists {
		t.Error("Expected Tmpfs to include /tmp mount")
	}
	if !strings.Contains(opts, "size=") {
		t.Errorf("Expected /tmp tmpfs to have size limit, got: %s", opts)
	}
	if !strings.Contains(opts, "noexec") {
		t.Errorf("Expected /tmp tmpfs to have noexec flag, got: %s", opts)
	}
	if !strings.Contains(opts, "nosuid") {
		t.Errorf("Expected /tmp tmpfs to have nosuid flag, got: %s", opts)
	}
}

func TestCreateContainer_ShouldHaveNoHostVolumeBinds(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		NetworkDisabled: true,
	})
	if len(capturedOpts.HostConfig.Binds) != 0 {
		t.Errorf("Expected no host volume binds for security, got: %v", capturedOpts.HostConfig.Binds)
	}
}

func TestCreateContainer_ShouldSetMemorySwapEqualToMemory(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		NetworkDisabled: true,
	})
	if capturedOpts.HostConfig.MemorySwap != capturedOpts.HostConfig.Memory {
		t.Errorf("Expected MemorySwap=%d to equal Memory=%d to prevent swap usage",
			capturedOpts.HostConfig.MemorySwap, capturedOpts.HostConfig.Memory)
	}
}

func TestCreateContainer_ShouldNotSetPrivilegedMode(t *testing.T) {
	var capturedOpts client.ContainerCreateOptions
	api := happyDockerAPI()
	api.containerCreateFn = func(_ context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
		capturedOpts = opts
		return client.ContainerCreateResult{ID: "test"}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, _ = rt.CreateContainer(context.Background(), SandboxContainerConfig{
		Image:           "python:3-slim",
		Cmd:             []string{"echo", "hi"},
		MemoryLimit:     defaultMemoryLimit,
		NetworkDisabled: true,
	})
	if capturedOpts.HostConfig.Privileged {
		t.Error("Expected Privileged=false for hardened isolation")
	}
}

// =============================================================================
// StartContainer
// =============================================================================

func TestStartContainer_ShouldSucceed(t *testing.T) {
	api := happyDockerAPI()
	rt := &DockerContainerRuntime{api: api}
	err := rt.StartContainer(context.Background(), "container-1")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
}

func TestStartContainer_ShouldReturnErrorWhenStartFails(t *testing.T) {
	api := happyDockerAPI()
	api.containerStartFn = func(_ context.Context, _ string, _ client.ContainerStartOptions) (client.ContainerStartResult, error) {
		return client.ContainerStartResult{}, errors.New("start failed")
	}
	rt := &DockerContainerRuntime{api: api}
	err := rt.StartContainer(context.Background(), "container-1")
	if err == nil {
		t.Fatal("Expected error when start fails")
	}
}

// =============================================================================
// WaitContainer
// =============================================================================

func TestWaitContainer_ShouldReturnExitCodeFromStatusChannel(t *testing.T) {
	api := happyDockerAPI()
	api.containerWaitFn = func(_ context.Context, _ string, _ client.ContainerWaitOptions) client.ContainerWaitResult {
		statusCh := make(chan container.WaitResponse, 1)
		errCh := make(chan error) // unbuffered, nothing sent
		statusCh <- container.WaitResponse{StatusCode: 42}
		return client.ContainerWaitResult{Result: statusCh, Error: errCh}
	}
	rt := &DockerContainerRuntime{api: api}
	code, err := rt.WaitContainer(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if code != 42 {
		t.Errorf("Expected exit code 42, got %d", code)
	}
}

func TestWaitContainer_ShouldReturnErrorFromErrorChannel(t *testing.T) {
	api := happyDockerAPI()
	api.containerWaitFn = func(_ context.Context, _ string, _ client.ContainerWaitOptions) client.ContainerWaitResult {
		statusCh := make(chan container.WaitResponse) // unbuffered
		errCh := make(chan error, 1)
		errCh <- errors.New("wait error")
		return client.ContainerWaitResult{Result: statusCh, Error: errCh}
	}
	rt := &DockerContainerRuntime{api: api}
	code, err := rt.WaitContainer(context.Background(), "c1")
	if err == nil {
		t.Fatal("Expected error from error channel")
	}
	if code != -1 {
		t.Errorf("Expected code -1, got %d", code)
	}
}

func TestWaitContainer_ShouldReturnNegativeOneWhenNilErrorOnErrorChannel(t *testing.T) {
	api := happyDockerAPI()
	api.containerWaitFn = func(_ context.Context, _ string, _ client.ContainerWaitOptions) client.ContainerWaitResult {
		statusCh := make(chan container.WaitResponse) // unbuffered, nothing sent
		errCh := make(chan error, 1)
		errCh <- nil // nil error on error channel
		return client.ContainerWaitResult{Result: statusCh, Error: errCh}
	}
	rt := &DockerContainerRuntime{api: api}
	code, err := rt.WaitContainer(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Expected nil error, got: %v", err)
	}
	if code != -1 {
		t.Errorf("Expected code -1 for nil-error path, got %d", code)
	}
}

func TestWaitContainer_ShouldReturnErrorWhenContextCancelled(t *testing.T) {
	api := happyDockerAPI()
	api.containerWaitFn = func(_ context.Context, _ string, _ client.ContainerWaitOptions) client.ContainerWaitResult {
		// No data on either channel — only ctx.Done() will fire
		return client.ContainerWaitResult{
			Result: make(chan container.WaitResponse),
			Error:  make(chan error),
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	rt := &DockerContainerRuntime{api: api}
	code, err := rt.WaitContainer(ctx, "c1")
	if err == nil {
		t.Fatal("Expected context error")
	}
	if code != -1 {
		t.Errorf("Expected code -1, got %d", code)
	}
}

// =============================================================================
// GetLogs
// =============================================================================

func TestGetLogs_ShouldReturnLogContent(t *testing.T) {
	api := happyDockerAPI()
	api.containerLogsFn = func(_ context.Context, _ string, _ client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
		return io.NopCloser(strings.NewReader("hello world\n")), nil
	}
	rt := &DockerContainerRuntime{api: api}
	logs, err := rt.GetLogs(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if logs != "hello world\n" {
		t.Errorf("Expected 'hello world\\n', got '%s'", logs)
	}
}

func TestGetLogs_ShouldReturnErrorWhenLogsFails(t *testing.T) {
	api := happyDockerAPI()
	api.containerLogsFn = func(_ context.Context, _ string, _ client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
		return nil, errors.New("logs unavailable")
	}
	rt := &DockerContainerRuntime{api: api}
	_, err := rt.GetLogs(context.Background(), "c1")
	if err == nil {
		t.Fatal("Expected error when logs fails")
	}
}

func TestGetLogs_ShouldReturnErrorWhenCopyFails(t *testing.T) {
	api := happyDockerAPI()
	api.containerLogsFn = func(_ context.Context, _ string, _ client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
		return &errReader{err: errors.New("copy error")}, nil
	}
	rt := &DockerContainerRuntime{api: api}
	_, err := rt.GetLogs(context.Background(), "c1")
	if err == nil {
		t.Fatal("Expected error when copy fails")
	}
}

// =============================================================================
// RemoveContainer
// =============================================================================

func TestRemoveContainer_ShouldSucceed(t *testing.T) {
	api := happyDockerAPI()
	rt := &DockerContainerRuntime{api: api}
	err := rt.RemoveContainer(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
}

func TestRemoveContainer_ShouldReturnErrorWhenRemoveFails(t *testing.T) {
	api := happyDockerAPI()
	api.containerRemoveFn = func(_ context.Context, _ string, _ client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
		return client.ContainerRemoveResult{}, errors.New("remove failed")
	}
	rt := &DockerContainerRuntime{api: api}
	err := rt.RemoveContainer(context.Background(), "c1")
	if err == nil {
		t.Fatal("Expected error when remove fails")
	}
}

// =============================================================================
// Close
// =============================================================================

func TestClose_ShouldSucceed(t *testing.T) {
	api := happyDockerAPI()
	rt := &DockerContainerRuntime{api: api}
	err := rt.Close()
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
}

func TestClose_ShouldReturnErrorWhenCloseFails(t *testing.T) {
	api := happyDockerAPI()
	api.closeFn = func() error { return errors.New("close failed") }
	rt := &DockerContainerRuntime{api: api}
	err := rt.Close()
	if err == nil {
		t.Fatal("Expected error when close fails")
	}
}

// =============================================================================
// dockerClientAdapter — exercises real adapter delegation for coverage
// =============================================================================

func newTestAdapter(t *testing.T) *dockerClientAdapter {
	t.Helper()
	httpClient := &http.Client{Transport: &failTransport{}}
	cli, err := client.NewClientWithOpts(
		client.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("cannot create test client: %v", err)
	}
	return &dockerClientAdapter{cli: cli}
}

func TestDockerClientAdapter_ImagePull_ShouldDelegateToClient(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.ImagePull(context.Background(), "test:latest", client.ImagePullOptions{})
	if err == nil {
		t.Error("Expected error from failing transport")
	}
}

func TestDockerClientAdapter_ContainerCreate_ShouldDelegateToClient(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.ContainerCreate(context.Background(), client.ContainerCreateOptions{
		Config: &container.Config{Image: "test"},
	})
	if err == nil {
		t.Error("Expected error from failing transport")
	}
}

func TestDockerClientAdapter_ContainerStart_ShouldDelegateToClient(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.ContainerStart(context.Background(), "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678", client.ContainerStartOptions{})
	if err == nil {
		t.Error("Expected error from failing transport")
	}
}

func TestDockerClientAdapter_ContainerWait_ShouldDelegateToClient(t *testing.T) {
	a := newTestAdapter(t)
	result := a.ContainerWait(context.Background(), "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678", client.ContainerWaitOptions{})
	// The error channel should eventually receive an error
	select {
	case err := <-result.Error:
		if err == nil {
			t.Error("Expected error from failing transport")
		}
	case <-result.Result:
		t.Error("Did not expect a result from failing transport")
	}
}

func TestDockerClientAdapter_ContainerLogs_ShouldDelegateToClient(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.ContainerLogs(context.Background(), "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678", client.ContainerLogsOptions{})
	if err == nil {
		t.Error("Expected error from failing transport")
	}
}

func TestDockerClientAdapter_ContainerRemove_ShouldDelegateToClient(t *testing.T) {
	a := newTestAdapter(t)
	_, err := a.ContainerRemove(context.Background(), "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678", client.ContainerRemoveOptions{})
	if err == nil {
		t.Error("Expected error from failing transport")
	}
}

func TestDockerClientAdapter_Close_ShouldDelegateToClient(t *testing.T) {
	a := newTestAdapter(t)
	// Close should succeed (client was never connected)
	err := a.Close()
	if err != nil {
		t.Errorf("Expected Close to succeed, got: %v", err)
	}
}

// =============================================================================
// Default newDockerClientFunc — exercises the real client factory
// =============================================================================

func TestDefaultNewDockerClientFunc_ShouldCreateClientSuccessfully(t *testing.T) {
	// The default newDockerClientFunc creates a real Docker client using
	// client.FromEnv. Client creation succeeds even without a running daemon
	// because it only configures an HTTP client (no connection is made).
	api, err := newDockerClientFunc()
	if err != nil {
		t.Fatalf("Expected client creation to succeed, got: %v", err)
	}
	if api == nil {
		t.Fatal("Expected non-nil API client")
	}
	if err := api.Close(); err != nil {
		t.Errorf("Expected Close to succeed, got: %v", err)
	}
}

func TestDefaultNewDockerClientFunc_ShouldReturnErrorWhenTLSConfigFails(t *testing.T) {
	// When DOCKER_TLS_VERIFY=1 but DOCKER_CERT_PATH points to a nonexistent
	// directory, FromEnv fails to load TLS certificates and returns an error.
	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", "/nonexistent/cert/path")

	api, err := newDockerClientFunc()
	if err == nil {
		// Some environments may not enforce TLS cert loading. In that case
		// just clean up and skip the assertion.
		if api != nil {
			api.Close()
		}
		t.Skip("Docker SDK did not fail with invalid TLS config in this environment")
	}
	if api != nil {
		t.Error("Expected nil API client on error")
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ dockerAPIClient = (*mockDockerAPI)(nil)
