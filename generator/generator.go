package generator

import (
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/util"
	"fmt"
	"github.com/gruntwork-io/boilerplate/errors"
	"path/filepath"
	"os"
	"io/ioutil"
	"strings"
	"path"
)

func Run(options *config.BoilerplateOptions, boilerplateConfig *config.BoilerplateConfig) error {
	variables, err := getVariables(options, boilerplateConfig)
	if err != nil {
		return err
	}

	return processTemplateFolder(options.TemplateFolder, options.OutputFolder, variables)
}

func getVariables(options *config.BoilerplateOptions, boilerplateConfig *config.BoilerplateConfig) (map[string]string, error) {
	variables := map[string]string{}

	for _, variable := range boilerplateConfig.Variables {
		value, err := getVariable(variable, options)
		if err != nil {
			return variables, err
		}
		variables[variable.Name] = value
	}

	return variables, nil
}

func getVariable(variable config.Variable, options *config.BoilerplateOptions) (string, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, options)

	if valueSpecifiedInVars {
		util.Logger.Printf("Using value specified via the --%s flag for variable '%s': %s", config.OPT_VAR, variable.Name, valueFromVars)
		return valueFromVars, nil
	} else if options.NonInteractive && variable.Default != "" {
		// TODO: how to disambiguate between a default not being specified and a default set to an empty string?
		util.Logger.Printf("Using default value for variable '%s': %s", variable.Name, variable.Default)
		return variable.Default, nil
	} else if options.NonInteractive {
		return "", errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.Name))
	} else {
		return getVariableFromUser(variable, options)
	}
}

func getVariableFromVars(variable config.Variable, options *config.BoilerplateOptions) (string, bool) {
	for name, value := range options.Vars {
		if name == variable.Name {
			return value, true
		}
	}

	return "", false
}

func getVariableFromUser(variable config.Variable, options *config.BoilerplateOptions) (string, error) {
	prompt := fmt.Sprintf("Enter a value for variable '%s'", variable.Name)

	if variable.Prompt != "" {
		prompt = variable.Prompt
	}
	if variable.Default != "" {
		prompt = fmt.Sprintf("%s (default: %s)", prompt, variable.Default)
	}

	value, err := util.PromptUserForInput(prompt)
	if err != nil {
		return "", err
	}

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		util.Logger.Printf("Using default value for variable '%s': %s", variable.Name, variable.Default)
		return variable.Default, nil
	} else {
		return value, nil
	}
}

func processTemplateFolder(templateFolder string, outputFolder string, variables map[string]string) error {
	util.Logger.Printf("Processing templates in %s and outputting generated files to %s", templateFolder, outputFolder)

	if err := os.MkdirAll(outputFolder, 0777); err != nil {
		return errors.WithStackTrace(err)
	}

	return filepath.Walk(templateFolder, func(path string, info os.FileInfo, err error) error {
		if shouldSkipPath(path, templateFolder) {
			util.Logger.Printf("Skipping %s", path)
			return nil
		} else {
			return processPath(path, templateFolder, outputFolder, variables)
		}
	})
}

func processPath(path string, templateFolder string, outputFolder string, variables map[string]string) error {
	isText, err := util.IsTextFile(path)
	if err != nil {
		return err
	}

	if isText {
		return processTemplate(path, templateFolder, outputFolder, variables)
	} else {
		return copyFile(path, templateFolder, outputFolder, variables)
	}
}

func outPath(file string, templateFolder string, outputFolder string) string {
	// TODO process template syntax in paths
	relativePath := strings.TrimPrefix(file, templateFolder)
	return path.Join(outputFolder, relativePath)
}

func copyFile(file string, templateFolder string, outputFolder string, variables map[string]string) error {
	destination := outPath(file, templateFolder, outputFolder)
	util.Logger.Printf("Copying %s to %s", file, destination)
	return util.CopyFile(file, destination)
}

func processTemplate(templatePath string, templateFolder string, outputFolder string, variables map[string]string) error {
	destination := outPath(templatePath, templateFolder, outputFolder)
	util.Logger.Printf("Processing template %s and writing to %s", templatePath, destination)

	bytes, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	out, err := RenderTemplate(templatePath, string(bytes), variables)
	if err != nil {
		return err
	}

	return util.WriteFileWithSamePermissions(templatePath, destination, []byte(out))
}

func shouldSkipPath(path string, templateFolder string) bool {
	return path == templateFolder || path == config.BoilerPlateConfigPath(templateFolder)
}

// Custom error types

type MissingVariableWithNonInteractiveMode string
func (variableName MissingVariableWithNonInteractiveMode) Error() string {
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", variableName, config.OPT_VAR, config.OPT_NON_INTERACTIVE)
}