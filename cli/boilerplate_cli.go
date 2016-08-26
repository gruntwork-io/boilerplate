package cli

import (
	"github.com/urfave/cli"
	"fmt"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/util"
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

	return templates.ProcessTemplateFolder(options, variables)
}

// Parse the command line options provided by the user
func parseOptions(cliContext *cli.Context) (*config.BoilerplateOptions, error) {
	vars, err := parseVars(cliContext.StringSlice(config.OPT_VAR), cliContext.StringSlice(config.OPT_VAR_FILE))
	if err != nil {
		return nil, err
	}

	missingKeyActionName := cliContext.String(config.OPT_MISSING_KEY_ACTION)
	missingKeyAction := config.DEFAULT_MISSING_KEY_ACTION
	if missingKeyActionName != "" {
		missingKeyAction, err = config.ParseMissingKeyAction(missingKeyActionName)
		if err != nil {
			return nil, err
		}
	}

	options := &config.BoilerplateOptions{
		TemplateFolder: cliContext.String(config.OPT_TEMPLATE_FOLDER),
		OutputFolder: cliContext.String(config.OPT_OUTPUT_FOLDER),
		NonInteractive: cliContext.Bool(config.OPT_NON_INTERACTIVE),
		OnMissingKey: missingKeyAction,
		Vars: vars,
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	return options, nil
}

// Parse variables passed in via command line flags, either as a list of NAME=VALUE variable pairs in varsList, or a
// list of paths to YAML files that define NAME: VALUE pairs. Return a map of the NAME: VALUE pairs.
func parseVars(varsList []string, varFileList[]string) (map[string]string, error) {
	variables := map[string]string{}

	varsFromVarsList, err := config.ParseVariablesFromKeyValuePairs(varsList)
	if err != nil {
		return variables, err
	}

	varsFromVarFiles, err := config.ParseVariablesFromVarFiles(varFileList)
	if err != nil {
		return variables, err
	}

	return util.MergeMaps(varsFromVarsList, varsFromVarFiles), nil
}

