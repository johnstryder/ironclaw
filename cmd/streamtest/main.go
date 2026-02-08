package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"ironclaw/internal/tooling"
)

// osExit wraps os.Exit so tests can capture exit calls.
var osExit = os.Exit

func main() {
	tool := tooling.NewShellToolWithStreaming(
		nil,
		&tooling.ExecCommandRunner{},
		&tooling.ExecStreamingCommandRunner{},
	)

	// Default script if no argument provided
	script := `
echo "=== Deploy Script v1.2 ==="
echo "  [1/5] Checking dependencies..."
sleep 0.4
echo "  [2/5] Running migrations..."
echo "    > ALTER TABLE users ADD COLUMN last_seen TIMESTAMP" >&2
sleep 0.5
echo "  [3/5] Building application..."
sleep 0.6
echo "    > warning: unused variable 'tmp' in worker.go:42" >&2
echo "  [4/5] Running test suite..."
sleep 0.3
echo "    PASS: auth_test (12 assertions)"
echo "    PASS: api_test  (8 assertions)"
echo "    PASS: db_test   (5 assertions)"
sleep 0.2
echo "  [5/5] Deploying to staging..."
sleep 0.5
echo "=== Deploy complete (exit 0) ==="
`
	if len(os.Args) > 1 {
		script = os.Args[1]
	}

	args, _ := json.Marshal(map[string]string{"command": script})

	fmt.Println("\033[1;36m--- Real-time streaming output ---\033[0m")
	fmt.Println()

	start := time.Now()
	lineCount := 0

	result, err := tool.CallStreaming(json.RawMessage(args), func(line tooling.OutputLine) {
		lineCount++
		elapsed := time.Since(start).Truncate(time.Millisecond)
		switch line.Source {
		case "stdout":
			fmt.Printf("\033[32m[%s stdout]\033[0m %s\n", elapsed, line.Line)
		case "stderr":
			fmt.Printf("\033[33m[%s stderr]\033[0m %s\n", elapsed, line.Line)
		}
	})

	fmt.Println()
	if err != nil {
		fmt.Printf("\033[1;31mERROR:\033[0m %v\n", err)
		osExit(1)
		return
	}

	fmt.Println("\033[1;36m--- Summary ---\033[0m")
	fmt.Printf("  Lines streamed : %d\n", lineCount)
	fmt.Printf("  Exit code      : %s\n", result.Metadata["exit_code"])
	fmt.Printf("  Mode           : %s\n", result.Metadata["mode"])
	fmt.Printf("  Wall time      : %s\n", time.Since(start).Truncate(time.Millisecond))
	fmt.Println()
	fmt.Println("\033[1;36m--- Final ToolResult.Data ---\033[0m")
	fmt.Println(result.Data)
}
