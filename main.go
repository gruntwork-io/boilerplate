package main

import (
	"os"
	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/errors"
)

// This variable is set at build time using -ldflags parameters. For more info, see:
// http://stackoverflow.com/a/11355611/483528
var VERSION string

// The main entrypoint for boilerplate
func main() {
	app := cli.CreateBoilerplateCli(VERSION)
	err := app.Run(os.Args)

	if err != nil {
		printError(err)
		os.Exit(1)
	}
}

// Display the given error in the console
func printError(err error) {
	if os.Getenv("BOILERPLATE_DEBUG") != "" {
		util.Logger.Println(errors.PrintErrorWithStackTrace(err))
	} else {
		util.Logger.Println(err)
	}
}