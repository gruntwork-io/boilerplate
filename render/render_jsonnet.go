package render

import (
	"encoding/json"

	jsonnet "github.com/google/go-jsonnet"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
)

// The following variables need to be stripped as it can't be rendered into json.
var incompatibleVariables = []string{
	"This",
}

// RenderJsonnetTemplate renders the jsonnet template at templatePath to the target location specified in the
// boilerplate options, passing in the boilerplate variables as Top Level Arguments. Note that this function will also
// make available the following helper values as External variables:
//
// - templateFolder
// - outputFolder
func RenderJsonnetTemplate(
	templatePath string,
	variables map[string]interface{},
	opts *options.BoilerplateOptions,
) (string, error) {
	jsonnetVM := jsonnet.MakeVM()
	configureExternalVars(opts, jsonnetVM)
	if err := configureTLAVarsFromBoilerplateVars(jsonnetVM, variables); err != nil {
		return "", err
	}
	output, err := jsonnetVM.EvaluateFile(templatePath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	return output, nil
}

// configureExternalVars registers the helper values as external variables for the jsonnet template engine.
func configureExternalVars(opts *options.BoilerplateOptions, vm *jsonnet.VM) {
	vm.ExtVar("templateFolder", opts.TemplateFolder)
	vm.ExtVar("outputFolder", opts.OutputFolder)
}

// configureTLAVarsFromBoilerplateVars translates the boilerplate variables into jsonnet values to make accessible to
// the top level function. Each boilerplate variable will be nested in an object boilerplateVars to avoid requiring
// every variable be defined.
// To pass through the boilerplate variables, we cheat by using json as an intermediary representation.
func configureTLAVarsFromBoilerplateVars(vm *jsonnet.VM, vars map[string]interface{}) error {
	// Some of the auto injected vars are not json marshable at the moment, so we skip those.
	jsonCompatibleMap := map[string]interface{}{}
	for k, v := range vars {
		if util.ListContains(k, incompatibleVariables) == false {
			jsonCompatibleMap[k] = v
		}
	}

	jsonBytes, err := json.Marshal(jsonCompatibleMap)
	if err != nil {
		return errors.WithStackTrace(err)
	}
	vm.TLACode("boilerplateVars", string(jsonBytes))
	return nil
}
