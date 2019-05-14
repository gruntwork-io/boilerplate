package cli

import (
	"fmt"

	"github.com/gruntwork-io/gruntwork-cli/entrypoint"
	"github.com/urfave/cli"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/variables"
)

const CUSTOM_HELP_TEXT = `Usage: {{.UsageText}}

A tool for generating files and folders (\"boilerplate\") from a set of templates. Examples:

Generate a project in ~/output from the templates in ~/templates:

    boilerplate --template-folder ~/templates --output-folder ~/output

Generate a project in ~/output from the templates in ~/templates, using variables passed in via the command line:

    boilerplate --template-folder ~/templates --output-folder ~/output --var "Title=Boilerplate" --var "ShowLogo=false"

Generate a project in ~/output from the templates in ~/templates, using variables read from a file:

    boilerplate --template-folder ~/templates --output-folder ~/output --var-file vars.yml


Options:

   {{range .VisibleFlags}}{{.}}
   {{end}}`

func CreateBoilerplateCli(version string) *cli.App {
	cli.HelpPrinter = entrypoint.WrappedHelpPrinter
	cli.AppHelpTemplate = CUSTOM_HELP_TEXT
	app := cli.NewApp()
	entrypoint.HelpTextLineWidth = 120

	app.Name = "boilerplate"
	app.Author = "Gruntwork <www.gruntwork.io>"
	app.UsageText = "boilerplate [OPTIONS]"
	app.Version = version
	app.Action = runApp

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  options.OPT_TEMPLATE_FOLDER,
			Usage: "Generate the project from the templates in `FOLDER`.",
		},
		cli.StringFlag{
			Name:  options.OPT_OUTPUT_FOLDER,
			Usage: "Create the output files and folders in `FOLDER`.",
		},
		cli.BoolFlag{
			Name:  options.OPT_NON_INTERACTIVE,
			Usage: fmt.Sprintf("Do not prompt for input variables. All variables must be set via --%s and --%s options instead.", options.OPT_VAR, options.OPT_VAR_FILE),
		},
		cli.StringSliceFlag{
			Name:  options.OPT_VAR,
			Usage: "Use `NAME=VALUE` to set variable NAME to VALUE. May be specified more than once.",
		},
		cli.StringSliceFlag{
			Name:  options.OPT_VAR_FILE,
			Usage: "Load variable values from the YAML file `FILE`. May be specified more than once.",
		},
		cli.StringFlag{
			Name:  options.OPT_MISSING_KEY_ACTION,
			Usage: fmt.Sprintf("What `ACTION` to take if a template looks up a variable that is not defined. Must be one of: %s. Default: %s.", options.ALL_MISSING_KEY_ACTIONS, options.DEFAULT_MISSING_KEY_ACTION),
		},
		cli.StringFlag{
			Name:  options.OPT_MISSING_CONFIG_ACTION,
			Usage: fmt.Sprintf("What `ACTION` to take if a the template folder does not contain a boilerplate.yml file. Must be one of: %s. Default: %s.", options.ALL_MISSING_CONFIG_ACTIONS, options.DEFAULT_MISSING_CONFIG_ACTION),
		},
		cli.BoolFlag{
			Name:  options.OPT_DISABLE_HOOKS,
			Usage: "If this flag is set, no hooks will execute.",
		},
		cli.BoolFlag{
			Name:  options.OPT_DISABLE_SHELL,
			Usage: "If this flag is set, no shell helpers will execute. They will instead return the text 'replace-me'.",
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

	opts, err := options.ParseOptions(cliContext)
	if err != nil {
		return err
	}

	// The root boilerplate.yml is not itself a dependency, so we pass an empty Dependency.
	emptyDep := variables.Dependency{}

	return templates.ProcessTemplate(opts, opts, emptyDep)
}
