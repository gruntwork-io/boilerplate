//go:build !(js && wasm)

package main

import (
	"fmt"
	"os"

	"github.com/gruntwork-io/boilerplate/cli"
)

// The main entrypoint for boilerplate
func main() {
	app := cli.CreateBoilerplateCli()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
