package config

import (
	"context"
	"fmt"
	"maps"
	"sort"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/hashicorp/go-multierror"
)

const MaxReferenceDepth = 20

// GetVariables gets a value for each of the variables specified in boilerplateConfig, other than those already in existingVariables.
// The value for a variable can come from the user (if the non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariables(l logging.Logger, opts *options.BoilerplateOptions, boilerplateConfig, rootBoilerplateConfig *BoilerplateConfig, thisDep *variables.Dependency) (map[string]any, error) {
	return GetVariablesWithContext(context.Background(), l, opts, boilerplateConfig, rootBoilerplateConfig, thisDep)
}

// GetVariablesWithContext collects variables from the user, variable defaults in the boilerplate.yml config, command line options, and environment
// variables. Variables in Boilerplate can can be used in both the Boilerplate config itself and in the templates.
//
// The value for a variable can come from the user (if the non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariablesWithContext(ctx context.Context, l logging.Logger, opts *options.BoilerplateOptions, boilerplateConfig, rootBoilerplateConfig *BoilerplateConfig, thisDep *variables.Dependency) (map[string]any, error) {
	renderedVariables := map[string]any{}

	// Add a variable for all variables contained in the root config file. This will allow Golang template users
	// to directly access these with an expression like "{{ .BoilerplateConfigVars.foo.Default }}"
	rootConfigVars := rootBoilerplateConfig.GetVariablesMap()
	renderedVariables["BoilerplateConfigVars"] = rootConfigVars

	// Add a variable for all dependencies contained in the root config file. This will allow Golang template users
	// to directly access these with an expression like "{{ .BoilerplateConfigDeps.foo.OutputFolder }}"
	rootConfigDeps := map[string]*variables.Dependency{}

	for i := range rootBoilerplateConfig.Dependencies {
		dep := &rootBoilerplateConfig.Dependencies[i]
		rootConfigDeps[dep.Name] = dep
	}

	renderedVariables["BoilerplateConfigDeps"] = rootConfigDeps

	// Add a variable for "the boilerplate template currently being processed".
	thisTemplateProps := map[string]any{}
	thisTemplateProps["Config"] = boilerplateConfig
	thisTemplateProps["Options"] = opts
	thisTemplateProps["CurrentDep"] = thisDep
	renderedVariables["This"] = thisTemplateProps

	// The variables up to this point don't need any additional processing as they are builtin. User defined variables
	// can reference and use Go template syntax, so we pass them through a rendering pipeline to ensure they are
	// evaluated to values that can be used in the rest of the templates.
	variablesToRender := map[string]any{}

	// Collect the variable values that have been passed in from the command line.
	maps.Copy(variablesToRender, opts.Vars)

	// Collect the variable values that are defined in the config and get the value.
	variablesInConfig := boilerplateConfig.GetVariablesMap()

	// Prior to prompting the user for the defined variable values, we sort the variables
	// by user-defined presentation order. Users may specify the order: value when defining
	// their variables in boilerplate.yml. This order value is an int, and it is used to
	// determine the relative ordering of the variables by key name
	// In essence, since Go maps do not preserve order for performance reasons, we need to
	// work around that and introduce a concept of use-defined ordering when prompting for
	// variables. We do this to provide a more coherent and easy to work with form filling
	// process for our Ref Arch customers

	// Create a slice of KeyOrderPairs, which we'll be able to sort according to order value
	keyAndOrderPairs := make([]KeyAndOrderPair, 0, len(variablesInConfig))

	// Pair the keys for each variable to its user-defined presentation order
	for key, variable := range variablesInConfig {
		kop := KeyAndOrderPair{
			Key:   key,
			Order: variable.Order(),
		}
		keyAndOrderPairs = append(keyAndOrderPairs, kop)
	}

	// Sort the KeyAndOrderPairs by their order value
	// N.B. this syntax of sort.Slice requires Go 1.18 or above!
	sort.Slice(keyAndOrderPairs, func(i, j int) bool {
		return keyAndOrderPairs[i].Order < keyAndOrderPairs[j].Order
	})

	// Now, instead of just iterating through the map keys naively,
	// iterate through the slice of KeyOrderPairs, which are sorted by order
	// which means that in each iteration of the loop, we can fetch the next variable
	// by looking up its key in the original config-provided variables map
	for _, keyOrderPair := range keyAndOrderPairs {
		variable := variablesInConfig[keyOrderPair.Key]

		unmarshalled, err := GetValueForVariable(l, variable, variablesInConfig, variablesToRender, opts, 0)
		if err != nil {
			return nil, err
		}

		variablesToRender[variable.Name()] = unmarshalled
	}

	// Pass all the user provided variables through a rendering pipeline to ensure they are evaluated down to
	// primitives.
	newlyRenderedVariables, err := render.RenderVariablesWithContext(ctx, l, opts, variablesToRender, renderedVariables)
	if err != nil {
		return nil, err
	}

	// Convert all the rendered variables to match the type definition in the boilerplate config.
	for _, variable := range variablesInConfig {
		renderedValue := newlyRenderedVariables[variable.Name()]

		renderedValueWithType, err := variables.ConvertType(renderedValue, variable)
		if err != nil {
			return nil, err
		}

		renderedVariables[variable.Name()] = renderedValueWithType
	}

	return renderedVariables, nil
}

func GetValueForVariable(
	l logging.Logger,
	variable variables.Variable,
	variablesInConfig map[string]variables.Variable,
	valuesForPreviousVariables map[string]any,
	opts *options.BoilerplateOptions,
	referenceDepth int,
) (any, error) {
	if referenceDepth > MaxReferenceDepth {
		return nil, CyclicalReference{VariableName: variable.Name(), ReferenceName: variable.Reference()}
	}

	value, alreadyExists := valuesForPreviousVariables[variable.Name()]
	if alreadyExists {
		return value, nil
	}

	if variable.Reference() != "" {
		refValue, refExists := valuesForPreviousVariables[variable.Reference()]
		if refExists {
			return refValue, nil
		}

		reference, containsReference := variablesInConfig[variable.Reference()]
		if !containsReference {
			return nil, MissingReference{VariableName: variable.Name(), ReferenceName: variable.Reference()}
		}

		return GetValueForVariable(l, reference, variablesInConfig, valuesForPreviousVariables, opts, referenceDepth+1)
	}

	// Run the value we receive from getVariable through validations, ensuring values provided by --var-files will also be checked
	value, err := getVariable(l, variable, opts)
	if err != nil {
		return value, err
	}

	var result *multierror.Error
	// Run the value through any defined validations for the variable
	for _, customValidation := range variable.Validations() {
		// Run the specific validation against the user-provided value and store it in the map
		err := runValidator(value, customValidation.Validator)
		result = multierror.Append(result, err)
	}

	return value, result.ErrorOrNil()
}

// Get a value for the given variable. The value can come from the user (if the non-interactive option isn't set), the
// default value in the config, or a command line option.
func getVariable(l logging.Logger, variable variables.Variable, opts *options.BoilerplateOptions) (any, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, opts)

	switch {
	case valueSpecifiedInVars:
		l.Debugf("Using value specified via command line options for variable '%s': %s", variable.FullName(), valueFromVars)
		return valueFromVars, nil
	case opts.NonInteractive && variable.Default() != nil:
		l.Debugf("Using default value for variable '%s': %v", variable.FullName(), variable.Default())
		return variable.Default(), nil
	case opts.NonInteractive:
		return nil, MissingVariableWithNonInteractiveMode(variable.FullName())
	case variable.Default() != nil && !variable.Confirm():
		l.Debugf("Using default value for variable '%s': %v", variable.FullName(), variable.Default())
		return variable.Default(), nil
	default:
		return getVariableFromUser(l, variable, variables.InvalidEntries{})
	}
}

// Return the value of the given variable from vars passed in as command line options
func getVariableFromVars(variable variables.Variable, opts *options.BoilerplateOptions) (any, bool) {
	for name, value := range opts.Vars {
		if name == variable.Name() {
			return value, true
		}
	}

	return nil, false
}

// KeyAndOrderPair is a composite of the user-defined order and the user's variable name
type KeyAndOrderPair struct {
	Key   string
	Order int
}

// Custom error types

type MissingVariableWithNonInteractiveMode string

func (variableName MissingVariableWithNonInteractiveMode) Error() string {
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", string(variableName), options.OptVar, options.OptNonInteractive)
}

type MissingReference struct {
	VariableName  string
	ReferenceName string
}

func (err MissingReference) Error() string {
	return fmt.Sprintf("Variable %s references unknown variable %s", err.VariableName, err.ReferenceName)
}

type CyclicalReference struct {
	VariableName  string
	ReferenceName string
}

func (err CyclicalReference) Error() string {
	return fmt.Sprintf("Variable %s seems to have an cyclical reference with variable %s", err.VariableName, err.ReferenceName)
}
