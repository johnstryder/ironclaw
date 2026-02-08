//go:build !excludemain

package main

import "os"

// exitFunc is the function used by main to exit; tests can replace it to cover main().
var exitFunc = os.Exit

func main() {
	exitFunc(runApp(os.Args))
}
