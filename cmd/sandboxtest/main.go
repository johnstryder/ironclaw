// sandboxtest is a manual integration test for the DockerSandboxTool.
// It requires a running Docker daemon.
//
// Usage:
//
//	go run ./cmd/sandboxtest/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"ironclaw/internal/tooling"
)

// runtimeFactory creates the ContainerRuntime. Package-level for test injection.
var runtimeFactory = func() (tooling.ContainerRuntime, io.Closer, error) {
	rt, err := tooling.NewDockerContainerRuntime()
	if err != nil {
		return nil, nil, err
	}
	return rt, rt, nil
}

// osExit wraps os.Exit so tests can capture exit calls.
var osExit = os.Exit

func main() {
	run(os.Stdout, os.Stderr)
}

func run(stdout, stderr io.Writer) {
	fmt.Fprintln(stdout, "=== Docker Sandbox Manual Integration Test ===")
	fmt.Fprintln(stdout, "Requires: Docker daemon running")
	fmt.Fprintln(stdout)

	rt, closer, err := runtimeFactory()
	if err != nil {
		fmt.Fprintf(stderr, "FAIL: Cannot connect to Docker: %v\n", err)
		fmt.Fprintf(stderr, "Make sure Docker is running: systemctl start docker\n")
		osExit(1)
		return
	}
	if closer != nil {
		defer closer.Close()
	}
	fmt.Fprintln(stdout, "[OK] Connected to Docker daemon")

	tool := tooling.NewDockerSandboxTool(rt)

	fmt.Fprintln(stdout, "\n--- Test 1: Python execution ---")
	runTest(stdout, tool, `{"language":"python","code":"print('Hello from sandbox!')"}`)

	fmt.Fprintln(stdout, "\n--- Test 2: Bash execution ---")
	runTest(stdout, tool, `{"language":"bash","code":"echo 'Hello from Alpine!' && whoami && hostname"}`)

	fmt.Fprintln(stdout, "\n--- Test 3: JavaScript execution ---")
	runTest(stdout, tool, `{"language":"javascript","code":"console.log('Hello from Node.js!'); console.log(process.version)"}`)

	fmt.Fprintln(stdout, "\n--- Test 4: Non-zero exit (Python error) ---")
	runTest(stdout, tool, `{"language":"python","code":"raise ValueError('intentional error')"}`)

	fmt.Fprintln(stdout, "\n--- Test 5: Network isolation (should fail) ---")
	runTest(stdout, tool, `{"language":"bash","code":"wget -T 3 http://google.com 2>&1 || echo 'GOOD: Network is blocked'"}`)

	fmt.Fprintln(stdout, "\n--- Test 6: Host isolation check ---")
	runTest(stdout, tool, `{"language":"bash","code":"cat /proc/1/cmdline 2>&1 || echo 'GOOD: Cannot read host PID 1'"}`)

	fmt.Fprintln(stdout, "\n--- Test 7: Multiline Python ---")
	code := "import sys\nfor i in range(5):\n    print(f'Line {i}')\nprint(f'Python {sys.version}')\n"
	payload := fmt.Sprintf(`{"language":"python","code":%s}`, jsonEscape(code))
	runTest(stdout, tool, payload)

	fmt.Fprintln(stdout, "\n--- Test 8: Custom timeout ---")
	runTest(stdout, tool, `{"language":"bash","code":"echo 'fast'","timeout":5}`)

	fmt.Fprintln(stdout, "\n--- Test 9: Input validation (should reject) ---")
	runTest(stdout, tool, `{"language":"cobol","code":"DISPLAY 'HI'"}`)

	fmt.Fprintln(stdout, "\n=== Manual Integration Test Complete ===")
}

func runTest(w io.Writer, tool *tooling.DockerSandboxTool, input string) {
	result, err := tool.Call(json.RawMessage(input))
	if err != nil {
		fmt.Fprintf(w, "  Error: %v\n", err)
		return
	}
	fmt.Fprintf(w, "  Output: %s\n", strings.TrimSpace(result.Data))
	fmt.Fprintf(w, "  Metadata: language=%s image=%s exit_code=%s\n",
		result.Metadata["language"],
		result.Metadata["image"],
		result.Metadata["exit_code"],
	)
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
