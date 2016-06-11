package cli

import (
	"github.com/urfave/cli"
	"fmt"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/templates"
	"strings"
	"github.com/gruntwork-io/boilerplate/errors"
)

// Customize the --help text for the app so we don't show extraneous info
const CUSTOM_HELP_TEXT = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}
   {{if .Version}}{{if not .HideVersion}}
VERSION:
   {{.Version}}
   {{end}}{{end}}{{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}{{if len .Authors}}
AUTHOR(S):
   {{range .Authors}}{{.}}{{end}}
   {{end}}{{if .Copyright}}
COPYRIGHT:
   {{.Copyright}}
   {{end}}
`

func CreateBoilerplateCli(version string) *cli.App {
	cli.AppHelpTemplate = CUSTOM_HELP_TEXT
	app := cli.NewApp()

	app.Name = "boilerplate"
	app.Author = "Gruntwork <www.gruntwork.io>"
	app.Usage = "A tool for generating files and folders (\"boilerplate\") from a set of templates"
	app.UsageText = "boilerplate [OPTIONS]"
	app.Version = version
	app.Action = runApp

	app.Flags = []cli.Flag {
		cli.StringFlag{
			Name: config.OPT_TEMPLATE_FOLDER,
			Usage: "Look for the project templates in `FOLDER`.",
		},
		cli.StringFlag{
			Name: config.OPT_OUTPUT_FOLDER,
			Usage: "Create the output files and folders in `FOLDER`.",
		},
		cli.BoolFlag{
			Name: config.OPT_NON_INTERACTIVE,
			Usage: fmt.Sprintf("Do not prompt for input variables. All variables must be set via --%s options instead.", config.OPT_VAR),
		},
		cli.StringSliceFlag{
			Name: config.OPT_VAR,
			Usage: "Use `NAME=VALUE` to set variable NAME is to VALUE. May be specified more than once.",
		},
	}

	return app

}

// When you run the CLI, this is the action function that gets called
func runApp(cliContext *cli.Context) error {
	if !cliContext.Args().Present() && cliContext.NumFlags() == 0 {
		cli.ShowAppHelp(cliContext)
		return nil
	}

	options, err := parseOptions(cliContext)
	if err != nil {
		return err
	}

	boilerplateConfig, err := config.LoadBoilerPlateConfig(options)
	if err != nil {
		return err
	}

	variables, err := config.GetVariables(options, boilerplateConfig)
	if err != nil {
		return err
	}

	return templates.ProcessTemplateFolder(options.TemplateFolder, options.OutputFolder, variables)
}

// Parse the command line options provided by the user
func parseOptions(cliContext *cli.Context) (*config.BoilerplateOptions, error) {
	vars, err := parseVars(cliContext.StringSlice(config.OPT_VAR))
	if err != nil {
		return nil, err
	}

	options := &config.BoilerplateOptions{
		TemplateFolder: cliContext.String(config.OPT_TEMPLATE_FOLDER),
		OutputFolder: cliContext.String(config.OPT_OUTPUT_FOLDER),
		NonInteractive: cliContext.Bool(config.OPT_NON_INTERACTIVE),
		Vars: vars,
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	return options, nil
}

// Parse a list of NAME=VALUE variable pairs into a map
func parseVars(varsList []string) (map[string] string, error) {
	vars := map[string]string{}

	for _, variable := range varsList {
		variableParts := strings.Split(variable, "=")
		if len(variableParts) != 2 {
			return vars, errors.WithStackTrace(InvalidVarSyntax(variable))
		}

		key := variableParts[0]
		value := variableParts[1]
		vars[key] = value
	}

	return vars, nil
}

// Custom error types

type InvalidVarSyntax string
func (varSyntax InvalidVarSyntax) Error() string {
	return fmt.Sprintf("Invalid syntax for variable. Expected NAME=VALUE but got %s", string(varSyntax))
}
