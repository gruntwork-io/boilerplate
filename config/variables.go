package config

import (
	"fmt"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/errors"
	"strings"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"reflect"
)

// A single variable defined in a boilerplate.yml config file
type Variable struct {
	Name        string
	Description string
	Type        BoilerplateType
	Default     interface{}
	Options     []string
}

// Return the full name of this variable, which includes its name and the dependency it is for (if any) in a
// human-readable format
func (variable Variable) FullName() string {
	dependencyName, variableName := SplitIntoDependencyNameAndVariableName(variable.Name)
	if dependencyName == "" {
		return variableName
	} else {
		return fmt.Sprintf("%s (for dependency %s)", variableName, dependencyName)
	}
}

func (variable Variable) String() string {
	return fmt.Sprintf("Variable {Name: '%s', Description: '%s', Type: '%s', Default: '%v', Options: '%v'}", variable.Name, variable.Description, variable.Type, variable.Default, variable.Options)
}

// Implement the go-yaml unmarshal interface for Variable. We can't let go-yaml handle this itself because we need to:
//
// 1. Set Defaults for missing fields (e.g. Type)
// 2. Validate the type corresponds to the Default value
// 3. Validate Options are only specified for the Enum Type
func (variable *Variable) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var fields map[string]interface{}
	if err := unmarshal(&fields); err != nil {
		return err
	}

	if unmarshalled, err := unmarshalVariable(fields); err != nil {
		return err
	} else {
		*variable = *unmarshalled
		return nil
	}
}

func unmarshalVariable(fields map[string]interface{}) (*Variable, error) {
	variable := Variable{}
	var err error

	variable.Name, err = unmarshalStringField(fields, "name", true, "")
	if err != nil {
		return nil, err
	}

	variable.Description, err = unmarshalStringField(fields, "description", false, variable.Name)
	if err != nil {
		return nil, err
	}

	variable.Type, err = unmarshalTypeField(fields, "type", variable.Name)
	if err != nil {
		return nil, err
	}

	variable.Options, err = unmarshalOptionsField(fields, "options", variable.Name, variable.Type)
	if err != nil {
		return nil, err
	}

	variable.Default, err = unmarshalValue(fields["default"], variable)
	if err != nil {
		return nil, err
	}

	return &variable, nil
}

func unmarshalValue(value interface{}, variable Variable) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch variable.Type {
	case String:
		if asString, isString := value.(string); isString {
			return asString, nil
		}
	case Int:
		if asInt, isInt := value.(int); isInt {
			return asInt, nil
		}
	case Float:
		if asFloat, isFloat := value.(float64); isFloat {
			return asFloat, nil
		}
	case Bool:
		if asBool, isBool := value.(bool); isBool {
			return asBool, nil
		}
	case List:
		if asList, isList := value.([]interface{}); isList {
			return toStringList(asList), nil
		}
	case Map:
		if asMap, isMap := value.(map[interface{}]interface{}); isMap {
			return toStringMap(asMap), nil
		}
	case Enum:
		if asString, isString := value.(string); isString {
			for _, option := range variable.Options {
				if asString == option {
					return asString, nil
				}
			}
		}
	}

	return nil, InvalidVariableValue{Variable: variable, Value: value}
}

func unmarshalOptionsField(fields map[string]interface{}, fieldName string, variableName string, variableType BoilerplateType) ([]string, error) {
	options, hasOptions := fields[fieldName]

	if !hasOptions {
		if variableType == Enum {
			return nil, errors.WithStackTrace(VariableMissingOptions(variableName))
		} else {
			return nil, nil
		}
	}

	if variableType != Enum {
		return nil, errors.WithStackTrace(OptionsCanOnlyBeUsedWithEnum{VariableName: variableName, VariableType: variableType})
	}

	optionsAsList, isList := options.([]interface{})
	if !isList {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "List", ActualType: reflect.TypeOf(options).String(), VariableName: variableName})
	}

	return toStringList(optionsAsList), nil
}

func toStringList(genericList []interface{}) []string {
	stringList := []string{}

	for _, value := range genericList {
		stringList = append(stringList, toString(value))
	}

	return stringList
}

func toStringMap(genericMap map[interface{}]interface{}) map[string]string {
	stringMap := map[string]string{}

	for key, value := range genericMap {
		stringMap[toString(key)] = toString(value)
	}

	return stringMap
}

func toString(value interface{}) string {
	return fmt.Sprintf("%v", value)
}

func unmarshalTypeField(fields map[string]interface{}, fieldName string, variableName string) (BoilerplateType, error) {
	variableTypeAsString, err := unmarshalStringField(fields, fieldName, false, variableName)
	if err != nil {
		return BOILERPLATE_TYPE_DEFAULT, err
	}

	if variableTypeAsString != "" {
		variableType, err := ParseBoilerplateType(variableTypeAsString)
		if err != nil {
			return BOILERPLATE_TYPE_DEFAULT, err
		}
		return *variableType, nil
	}

	return BOILERPLATE_TYPE_DEFAULT, nil
}

func unmarshalStringField(fields map[string]interface{}, fieldName string, requiredField bool, variableName string) (string, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return "", errors.WithStackTrace(RequiredFieldMissing(fieldName))
		} else {
			return "", nil
		}
	}

	if valueAsString, isString := value.(string); isString {
		return valueAsString, nil
	} else {
		return "", errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(value).String(), VariableName: variableName})
	}
}

// Get a value for each of the variables specified in boilerplateConfig, other than those already in existingVariables.
// The value for a variable can come from the user (if the  non-interactive option isn't set), the default value in the
// config, or a command line option.
func GetVariables(options *BoilerplateOptions, boilerplateConfig *BoilerplateConfig) (map[string]interface{}, error) {
	variables := map[string]interface{}{}
	for key, value := range options.Vars {
		variables[key] = value
	}

	variablesInConfig := getAllVariablesInConfig(boilerplateConfig)

	for _, variable := range variablesInConfig {
		var value interface{}
		var err error

		value, alreadyExists := variables[variable.Name]
		if !alreadyExists {
			value, err = getVariable(variable, options)
			if err != nil {
				return variables, err
			}
		}

		variables[variable.Name], err = unmarshalValue(value, variable)
		if err != nil {
			return variables, err
		}
	}

	return variables, nil
}

// Get all the variables defined in the given config and its dependencies
func getAllVariablesInConfig(boilerplateConfig *BoilerplateConfig) []Variable {
	allVariables := []Variable{}

	allVariables = append(allVariables, boilerplateConfig.Variables...)

	for _, dependency := range boilerplateConfig.Dependencies {
		for _, variable := range dependency.Variables {
			variableName := fmt.Sprintf("%s.%s", dependency.Name, variable.Name)
			allVariables = append(allVariables, Variable{Name: variableName, Description: variable.Description, Type: variable.Type, Default: variable.Default, Options: variable.Options})
		}
	}

	return allVariables
}

// Get a value for the given variable. The value can come from the user (if the non-interactive option isn't set), the
// default value in the config, or a command line option.
func getVariable(variable Variable, options *BoilerplateOptions) (interface{}, error) {
	valueFromVars, valueSpecifiedInVars := getVariableFromVars(variable, options)

	if valueSpecifiedInVars {
		util.Logger.Printf("Using value specified via command line options for variable '%s': %s", variable.FullName(), valueFromVars)
		return valueFromVars, nil
	} else if options.NonInteractive && variable.Default != nil {
		// TODO: how to disambiguate between a default not being specified and a default set to an empty string?
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default)
		return variable.Default, nil
	} else if options.NonInteractive {
		return nil, errors.WithStackTrace(MissingVariableWithNonInteractiveMode(variable.FullName()))
	} else {
		return getVariableFromUser(variable, options)
	}
}

// Return the value of the given variable from vars passed in as command line options
func getVariableFromVars(variable Variable, options *BoilerplateOptions) (interface{}, bool) {
	for name, value := range options.Vars {
		if name == variable.Name {
			return value, true
		}
	}

	return nil, false
}

// Get the value for the given variable by prompting the user
func getVariableFromUser(variable Variable, options *BoilerplateOptions) (interface{}, error) {
	util.BRIGHT_GREEN.Printf("\n%s\n", variable.FullName())
	if variable.Description != "" {
		fmt.Printf("  %s\n", variable.Description)
	}
	if variable.Default != "" {
		fmt.Printf("  (default: %s)\n", variable.Default)
	}
	fmt.Println()

	value, err := util.PromptUserForInput("  Enter a value")
	if err != nil {
		return "", err
	}

	if value == "" {
		// TODO: what if the user wanted an empty string instead of the default?
		util.Logger.Printf("Using default value for variable '%s': %v", variable.FullName(), variable.Default)
		return variable.Default, nil
	}

	return parseYamlString(value)
}

// Parse a list of NAME=VALUE pairs into a map.
func ParseVariablesFromKeyValuePairs(varsList []string) (map[string]interface{}, error) {
	vars := map[string]interface{}{}

	for _, variable := range varsList {
		variableParts := strings.Split(variable, "=")
		if len(variableParts) != 2 {
			return vars, errors.WithStackTrace(InvalidVarSyntax(variable))
		}

		key := variableParts[0]
		value := variableParts[1]
		if key == "" {
			return vars, errors.WithStackTrace(VariableNameCannotBeEmpty(variable))
		}

		parsedValue, err := parseYamlString(value)
		if err != nil {
			return vars, err
		}

		vars[key] = parsedValue
	}

	return vars, nil
}

// Parse a YAML string into a Go type
func parseYamlString(str string) (interface{}, error) {
	var parsedValue interface{}

	err := yaml.Unmarshal([]byte(str), &parsedValue)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	return parsedValue, nil
}

// Parse a list of YAML files that define variables into a map.
func ParseVariablesFromVarFiles(varFileList []string) (map[string]interface{}, error) {
	vars := map[string]interface{}{}

	for _, varFile := range varFileList {
		varsInFile, err := ParseVariablesFromVarFile(varFile)
		if err != nil {
			return vars, err
		}
		vars = util.MergeMaps(vars, varsInFile)
	}

	return vars, nil
}

// Parse the NAME: VALUE pairs in the given YAML file into a map
func ParseVariablesFromVarFile(varFilePath string) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile(varFilePath)
	if err != nil {
		return map[string]interface{}{}, errors.WithStackTrace(err)
	}
	return parseVariablesFromVarFileContents(bytes)
}

// Parse the NAME: VALUE pairs in the given YAML file contents into a map
func parseVariablesFromVarFileContents(varFileContents []byte)(map[string]interface{}, error) {
	vars := map[string]interface{}{}

	err := yaml.Unmarshal(varFileContents, &vars)
	if err != nil {
		return vars, errors.WithStackTrace(err)
	}

	return vars, nil
}

// Custom error types

type MissingVariableWithNonInteractiveMode string
func (variableName MissingVariableWithNonInteractiveMode) Error() string {
	return fmt.Sprintf("Variable '%s' does not have a default, no value was specified at the command line using the --%s option, and the --%s flag is set, so cannot prompt user for a value.", string(variableName), OPT_VAR, OPT_NON_INTERACTIVE)
}

type InvalidVarSyntax string
func (varSyntax InvalidVarSyntax) Error() string {
	return fmt.Sprintf("Invalid syntax for variable. Expected NAME=VALUE but got %s", string(varSyntax))
}

type VariableNameCannotBeEmpty string
func (varSyntax VariableNameCannotBeEmpty) Error() string {
	return fmt.Sprintf("Variable name cannot be empty. Expected NAME=VALUE but got %s", string(varSyntax))
}

type InvalidVariableValue struct {
	Value    interface{}
	Variable Variable
}
func (err InvalidVariableValue) Error() string {
	message := fmt.Sprintf("Value '%v' is not a valid value for variable '%s' with type '%s'.", err.Value, err.Variable.Name, err.Variable.Type.String())
	if err.Variable.Type == Enum {
		message = fmt.Sprintf("%s. Value must be one of: %s.", message, err.Variable.Options)
	}
	return message
}

type InvalidTypeForField struct {
	FieldName string
	VariableName string
	ExpectedType string
	ActualType string
}
func (err InvalidTypeForField) Error() string {
	message := fmt.Sprintf("%s must have type %s but got %s", err.FieldName, err.ExpectedType, err.ActualType)
	if err.VariableName != "" {
		message = fmt.Sprintf("%s for variable %s", message, err.VariableName)
	}
	return message
}