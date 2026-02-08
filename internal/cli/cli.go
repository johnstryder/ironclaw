package cli

import (
	"fmt"
	"io"
)

// NailPolish is the CLI subcommand emoji for "nail polish" mode.
const NailPolish = "💅🏼"

// Run runs the CLI with the given args. Handles subcommands: check, 💅🏼.
// Returns 0 or 1 for check/emoji; returns 1 to mean "continue with normal startup".
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) >= 2 && args[1] == checkCmd {
		return RunCheck(args, stdout, stderr)
	}
	for _, a := range args {
		if a == NailPolish {
			fmt.Fprintln(stdout, NailPolish)
			return 0
		}
	}
	return 1
}
