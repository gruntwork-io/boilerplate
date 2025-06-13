package runbook

import (
	"github.com/gruntwork-io/boilerplate/options"
)

// LaunchRunbook starts the web server that renders the Runbook UI (and collects variables from the user).
func LaunchRunbook(opts *options.BoilerplateOptions) error {
	return StartRunbookServer(opts)
}
