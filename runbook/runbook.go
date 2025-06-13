package runbook

import (
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/pkg/browser"
)

// LaunchRunbook starts the web server that renders the Runbook UI (and collects variables from the user).
func LaunchRunbook(opts *options.BoilerplateOptions) error {
	// TODO: This is opening the frontend server, but the backend server might have failed. Handle this better..
	if err := browser.OpenURL("http://localhost:8080/form"); err != nil {
		return errors.WithStackTraceAndPrefix(err, "Failed to open browser")
	}

	if err := StartBackendServer(opts); err != nil {
		return errors.WithStackTraceAndPrefix(err, "Failed to start backend server")
	}

	return nil
}
