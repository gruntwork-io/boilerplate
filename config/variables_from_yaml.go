package config

import (
	"github.com/gruntwork-io/boilerplate/errors"
	"reflect"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"github.com/gruntwork-io/boilerplate/util"
	"strings"
	"fmt"
)

// Given a map where the keys are the fields of a boilerplate Variable, this method crates a Variable struct with those
// fields filled in with proper types. This method also validates all the fields and returns an error if any problems
// are found.
func UnmarshalVariable(fields map[string]interface{}) (*Variable, error) {
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

	variable.Default, err = UnmarshalVariableValue(fields["default"], variable)
	if err != nil {
		return nil, err
	}

	return &variable, nil
}

// Convert the given value to the proper type for the given variable, or return an error if the type doesn't match.
// For example, if this variable is of type List, then the returned value will be a list of strings.
func UnmarshalVariableValue(value interface{}, variable Variable) (interface{}, error) {
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
			return util.ToStringList(asList), nil
		}
	case Map:
		if asMap, isMap := value.(map[interface{}]interface{}); isMap {
			return util.ToStringMap(asMap), nil
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

// Extract the options field from the given map of fields using the given field name and convert those options to a
// list of strings.
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

	return util.ToStringList(optionsAsList), nil
}

// Extract the type field from the map of fields using the given field name and convert the type to a BoilerplateType.
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

// Extract a string field from the map of fields using the given field name and convert it to a string. If no such
// field is in the map of fields but requiredField is set to true, return an error.
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

// Parse a list of NAME=VALUE pairs passed in as command-line options into a map of variable names to variable values.
// Along the way, each value is parsed as YAML.
func parseVariablesFromKeyValuePairs(varsList []string) (map[string]interface{}, error) {
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

// Parse a list of YAML files that define variables into a map from variable name to variable value. Along the way,
// each value is parsed as YAML.
func parseVariablesFromVarFiles(varFileList []string) (map[string]interface{}, error) {
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

// Parse the variables in the given YAML file into a map of variable name to variable value. Along the way, each value
// is parsed as YAML.
func ParseVariablesFromVarFile(varFilePath string) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile(varFilePath)
	if err != nil {
		return map[string]interface{}{}, errors.WithStackTrace(err)
	}
	return parseVariablesFromVarFileContents(bytes)
}

// Parse the variables in the given YAML contents into a map of variable name to variable value. Along the way, each
// value is parsed as YAML.
func parseVariablesFromVarFileContents(varFileContents []byte)(map[string]interface{}, error) {
	vars := map[string]interface{}{}

	err := yaml.Unmarshal(varFileContents, &vars)
	if err != nil {
		return vars, errors.WithStackTrace(err)
	}

	return vars, nil
}


// Parse variables passed in via command line options, either as a list of NAME=VALUE variable pairs in varsList, or a
// list of paths to YAML files that define NAME: VALUE pairs. Return a map of the NAME: VALUE pairs. Along the way,
// each VALUE is parsed as YAML.
func parseVars(varsList []string, varFileList[]string) (map[string]interface{}, error) {
	variables := map[string]interface{}{}

	varsFromVarsList, err := parseVariablesFromKeyValuePairs(varsList)
	if err != nil {
		return variables, err
	}

	varsFromVarFiles, err := parseVariablesFromVarFiles(varFileList)
	if err != nil {
		return variables, err
	}

	return util.MergeMaps(varsFromVarsList, varsFromVarFiles), nil
}

// Custom error types

type VariableMissingOptions string
func (err VariableMissingOptions) Error() string {
	return fmt.Sprintf("Variable %s has type %s but does not specify any options. You must specify at least one option.", string(err), Enum)
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

type OptionsCanOnlyBeUsedWithEnum struct {
	VariableName string
	VariableType BoilerplateType
}
func (err OptionsCanOnlyBeUsedWithEnum) Error() string {
	return fmt.Sprintf("Variable %s has type %s and tries to specify options. Options may only be specified for variables of type %s.", err.VariableName, err.VariableType.String(), Enum)
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

type VariableNameCannotBeEmpty string
func (varSyntax VariableNameCannotBeEmpty) Error() string {
	return fmt.Sprintf("Variable name cannot be empty. Expected NAME=VALUE but got %s", string(varSyntax))
}

type InvalidVarSyntax string
func (varSyntax InvalidVarSyntax) Error() string {
	return fmt.Sprintf("Invalid syntax for variable. Expected NAME=VALUE but got %s", string(varSyntax))
}

type RequiredFieldMissing string
func (err RequiredFieldMissing) Error() string {
	return fmt.Sprintf("Variable is missing required field %s", string(err))
}