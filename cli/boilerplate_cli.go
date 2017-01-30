package cli

import (
	"github.com/urfave/cli"
	"fmt"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/variables"
)

// Customize the --help text for the app so we don't show extraneous info
const CUSTOM_HELP_TEXT = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}
   {{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}{{if len .Authors}}
EXAMPLES:
   Generate a project in ~/output from the templates in ~/templates:

       boilerplate --template-folder ~/templates --output-folder ~/output

   Generate a project in ~/output from the templates in ~/templates, using variables passed in via the command line:

       boilerplate --template-folder ~/templates --output-folder ~/output --var "Title=Boilerplate" --var "ShowLogo=false"

   Generate a project in ~/output from the templates in ~/templates, using variables read from a file:

       boilerplate --template-folder ~/templates --output-folder ~/output --var-file vars.yml

   {{if .Version}}{{if not .HideVersion}}
VERSION:
   {{.Version}}
   {{end}}{{end}}
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
			Usage: "Generate the project from the templates in `FOLDER`.",
		},
		cli.StringFlag{
			Name: config.OPT_OUTPUT_FOLDER,
			Usage: "Create the output files and folders in `FOLDER`.",
		},
		cli.BoolFlag{
			Name: config.OPT_NON_INTERACTIVE,
			Usage: fmt.Sprintf("Do not prompt for input variables. All variables must be set via --%s and --%s options instead.", config.OPT_VAR, config.OPT_VAR_FILE),
		},
		cli.StringSliceFlag{
			Name: config.OPT_VAR,
			Usage: "Use `NAME=VALUE` to set variable NAME to VALUE. May be specified more than once.",
		},
		cli.StringSliceFlag{
			Name: config.OPT_VAR_FILE,
			Usage: "Load variable values from the YAML file `FILE`. May be specified more than once.",
		},
		cli.StringFlag{
			Name: config.OPT_MISSING_KEY_ACTION,
			Usage: fmt.Sprintf("What `ACTION` to take if a template looks up a variable that is not defined. Must be one of: %s. Default: %s.", config.ALL_MISSING_KEY_ACTIONS, config.DEFAULT_MISSING_KEY_ACTION),
		},
		cli.StringFlag{
			Name: config.OPT_MISSING_CONFIG_ACTION,
			Usage: fmt.Sprintf("What `ACTION` to take if a the template folder does not contain a boilerplate.yml file. Must be one of: %s. Default: %s.", config.ALL_MISSING_CONFIG_ACTIONS, config.DEFAULT_MISSING_CONFIG_ACTION),
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

	options, err := config.ParseOptions(cliContext)
	if err != nil {
		return err
	}

	// The root boilerplate.yml is not itself a dependency, so we pass an empty Dependency.
	emptyDep := variables.Dependency{}

	return templates.ProcessTemplate(options, options, emptyDep)
}
