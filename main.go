package main

import (
	"github.com/gruntwork-io/go-commons/entrypoint"

	"github.com/gruntwork-io/boilerplate/cli"
)

// The main entrypoint for boilerplate
func main() {
	app := cli.CreateBoilerplateCli()
	entrypoint.RunApp(app)
}
