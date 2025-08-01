package testutil

import (
	"github.com/gruntwork-io/boilerplate/options"
)

// CreateTestOptions creates a BoilerplateOptions instance with common test settings
func CreateTestOptions(templateFolder string) *options.BoilerplateOptions {
	return &options.BoilerplateOptions{
		TemplateFolder:          templateFolder,
		NonInteractive:          true,
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
		ShellCommandAnswers:     make(map[string]bool),
		ExecuteAllShellCommands: false,
	}
}

// CreateTestOptionsWithOutput creates a BoilerplateOptions instance with template and output folders
func CreateTestOptionsWithOutput(templateFolder, outputFolder string) *options.BoilerplateOptions {
	return &options.BoilerplateOptions{
		TemplateFolder:          templateFolder,
		OutputFolder:            outputFolder,
		NonInteractive:          true,
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		NoHooks:                 true,
		DisableDependencyPrompt: true,
		ShellCommandAnswers:     make(map[string]bool),
		ExecuteAllShellCommands: false,
	}
}

// CreateTestOptionsForShell creates a BoilerplateOptions instance specifically for shell tests
func CreateTestOptionsForShell(nonInteractive bool, noShell bool) *options.BoilerplateOptions {
	return &options.BoilerplateOptions{
		NonInteractive:          nonInteractive,
		NoShell:                 noShell,
		ShellCommandAnswers:     make(map[string]bool),
		ExecuteAllShellCommands: false,
	}
}
