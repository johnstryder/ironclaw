package tooling

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"ironclaw/internal/domain"
)

// defaultTimeout is the default execution timeout in seconds when not specified.
const defaultTimeout = 10

// defaultMemoryLimit is the default memory limit for sandboxed containers (64 MB).
const defaultMemoryLimit int64 = 64 * 1024 * 1024

// defaultCPULimit is the default CPU limit for sandboxed containers in NanoCPUs (0.5 CPU).
const defaultCPULimit int64 = 500_000_000

// defaultPidsLimit is the default maximum number of processes in sandboxed containers.
const defaultPidsLimit int64 = 64

// DockerSandboxInput represents the input structure for sandbox code execution.
type DockerSandboxInput struct {
	Language string `json:"language" jsonschema:"enum=python,enum=bash,enum=javascript"`
	Code     string `json:"code" jsonschema:"minLength=1"`
	Timeout  int    `json:"timeout,omitempty" jsonschema:"minimum=1,maximum=30"`
}

// SandboxContainerConfig holds configuration for creating a sandboxed container.
type SandboxContainerConfig struct {
	Image           string
	Cmd             []string
	MemoryLimit     int64 // bytes
	CPULimit        int64 // NanoCPUs (10^-9 CPUs)
	PidsLimit       int64 // maximum number of processes
	NetworkDisabled bool
}

// ContainerRuntime abstracts Docker container lifecycle operations for testability.
// "Mock the Brain, Test the Body" — this interface is what we mock in tests.
type ContainerRuntime interface {
	EnsureImage(ctx context.Context, image string) error
	CreateContainer(ctx context.Context, cfg SandboxContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	WaitContainer(ctx context.Context, containerID string) (int64, error)
	GetLogs(ctx context.Context, containerID string) (string, error)
	RemoveContainer(ctx context.Context, containerID string) error
}

// languageImages maps supported languages to their Docker images.
var languageImages = map[string]string{
	"python":     "python:3-slim",
	"bash":       "alpine:latest",
	"javascript": "node:20-slim",
}

// languageInterpreters maps supported languages to their interpreter commands.
var languageInterpreters = map[string]string{
	"python":     "python3",
	"bash":       "sh",
	"javascript": "node",
}

// dockerSandboxUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth error path.
var dockerSandboxUnmarshalFunc = json.Unmarshal

// DockerSandboxTool executes user code in an isolated Docker container.
type DockerSandboxTool struct {
	runtime ContainerRuntime
}

// NewDockerSandboxTool creates a DockerSandboxTool with the given container runtime.
func NewDockerSandboxTool(runtime ContainerRuntime) *DockerSandboxTool {
	return &DockerSandboxTool{runtime: runtime}
}

// Name returns the tool name used in function-calling.
func (d *DockerSandboxTool) Name() string { return "docker_sandbox" }

// Description returns a human-readable description for the LLM.
func (d *DockerSandboxTool) Description() string {
	return "Executes code in a secure, isolated Docker sandbox container with no network access"
}

// Definition returns the JSON Schema for sandbox input.
func (d *DockerSandboxTool) Definition() string {
	return GenerateSchema(DockerSandboxInput{})
}

// Call validates the input and executes code in a sandboxed Docker container.
func (d *DockerSandboxTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := d.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input DockerSandboxInput
	if err := dockerSandboxUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Resolve image and command for the language
	image, ok := languageImages[input.Language]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", input.Language)
	}

	cmd := buildContainerCmd(input.Language, input.Code)
	timeout := resolveTimeout(input.Timeout)

	// 4. Create timeout context for the entire container lifecycle
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 5. Ensure the Docker image is available
	if err := d.runtime.EnsureImage(ctx, image); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	// 6. Create the sandboxed container
	cfg := SandboxContainerConfig{
		Image:           image,
		Cmd:             cmd,
		MemoryLimit:     defaultMemoryLimit,
		CPULimit:        defaultCPULimit,
		PidsLimit:       defaultPidsLimit,
		NetworkDisabled: true,
	}
	containerID, err := d.runtime.CreateContainer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// 7. Always clean up: remove the container when done (use background ctx so cleanup succeeds)
	defer d.runtime.RemoveContainer(context.Background(), containerID)

	// 8. Start the container
	if err := d.runtime.StartContainer(ctx, containerID); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// 9. Wait for the container to finish
	exitCode, err := d.runtime.WaitContainer(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for container: %w", err)
	}

	// 10. Retrieve logs (use background ctx — container is already stopped)
	logs, err := d.runtime.GetLogs(context.Background(), containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve logs: %w", err)
	}

	// 11. Return result with metadata
	return &domain.ToolResult{
		Data: logs,
		Metadata: map[string]string{
			"language":  input.Language,
			"image":     image,
			"exit_code": fmt.Sprintf("%d", exitCode),
		},
	}, nil
}

// buildContainerCmd constructs the container command for the given language and code.
// The code is base64-encoded to avoid shell escaping issues.
func buildContainerCmd(language, code string) []string {
	encoded := base64.StdEncoding.EncodeToString([]byte(code))
	interpreter := languageInterpreters[language]
	return []string{
		"sh", "-c",
		fmt.Sprintf("echo '%s' | base64 -d | %s", encoded, interpreter),
	}
}

// resolveTimeout returns the effective timeout duration, defaulting if not set.
func resolveTimeout(timeout int) time.Duration {
	if timeout <= 0 {
		return time.Duration(defaultTimeout) * time.Second
	}
	return time.Duration(timeout) * time.Second
}
