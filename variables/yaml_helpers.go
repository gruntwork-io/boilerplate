package variables

import (
	"github.com/gruntwork-io/boilerplate/errors"
	"reflect"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"github.com/gruntwork-io/boilerplate/util"
	"strings"
	"fmt"
)

func unmarshalListOfFields(fields map[string]interface{}, fieldName string) ([]map[string]interface{}, error) {
	listOfFields := []map[string]interface{}{}

	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return listOfFields, nil
	}

	asYamlList, isYamlList := fieldAsYaml.([]interface{})
	if !isYamlList {
		return listOfFields, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "[]interface{}", ActualType: reflect.TypeOf(fieldAsYaml)})
	}

	for _, asYaml := range asYamlList {
		asYamlMap, isYamlMap := asYaml.(map[interface{}]interface{})
		if !isYamlMap {
			return listOfFields, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "map[string]interface{}", ActualType: reflect.TypeOf(asYaml)})
		}

		listOfFields = append(listOfFields, util.ToStringToGenericMap(asYamlMap))
	}

	return listOfFields, nil
}

// Extract the options field from the given map of fields using the given field name and convert those options to a
// list of strings.
func unmarshalOptionsField(fields map[string]interface{}, fieldName string, context string, variableType BoilerplateType) ([]string, error) {
	options, hasOptions := fields[fieldName]

	if !hasOptions {
		if variableType == Enum {
			return nil, errors.WithStackTrace(OptionsMissing(context))
		} else {
			return nil, nil
		}
	}

	if variableType != Enum {
		return nil, errors.WithStackTrace(OptionsCanOnlyBeUsedWithEnum{Context: context, Type: variableType})
	}

	optionsAsList, isList := options.([]interface{})
	if !isList {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "List", ActualType: reflect.TypeOf(options), Context: context})
	}

	return util.ToStringList(optionsAsList), nil
}

// Extract the type field from the map of fields using the given field name and convert the type to a BoilerplateType.
func unmarshalTypeField(fields map[string]interface{}, fieldName string, context string) (BoilerplateType, error) {
	variableTypeAsString, err := unmarshalStringField(fields, fieldName, false, context)
	if err != nil {
		return BOILERPLATE_TYPE_DEFAULT, err
	}

	if variableTypeAsString != nil {
		variableType, err := ParseBoilerplateType(*variableTypeAsString)
		if err != nil {
			return BOILERPLATE_TYPE_DEFAULT, err
		}
		return *variableType, nil
	}

	return BOILERPLATE_TYPE_DEFAULT, nil
}

// Extract a string field from the map of fields using the given field name and convert it to a string. If no such
// field is in the map of fields but requiredField is set to true, return an error.
func unmarshalStringField(fields map[string]interface{}, fieldName string, requiredField bool, context string) (*string, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return nil, errors.WithStackTrace(RequiredFieldMissing(fieldName))
		} else {
			return nil, nil
		}
	}

	if valueAsString, isString := value.(string); isString {
		return &valueAsString, nil
	} else {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(value), Context: context})
	}
}

// Extract a boolean field from the map of fields using the given field name and convert it to a bool. If no such
// field is in the map of fields but requiredField is set to true, return an error.
func unmarshalBooleanField(fields map[string]interface{}, fieldName string, requiredField bool, context string) (bool, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return false, errors.WithStackTrace(RequiredFieldMissing(fieldName))
		} else {
			return false, nil
		}
	}

	if valueAsBool, isBool:= value.(bool); isBool {
		return valueAsBool, nil
	} else {
		return false, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "bool", ActualType: reflect.TypeOf(value), Context: context})
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

		parsedValue, err := ParseYamlString(value)
		if err != nil {
			return vars, err
		}

		vars[key] = parsedValue
	}

	return vars, nil
}

// Parse a YAML string into a Go type
func ParseYamlString(str string) (interface{}, error) {
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
func ParseVars(varsList []string, varFileList[]string) (map[string]interface{}, error) {
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

type OptionsMissing string
func (err OptionsMissing) Error() string {
	return fmt.Sprintf("%s has type %s but does not specify any options. You must specify at least one option.", string(err), Enum)
}

type InvalidVariableValue struct {
	Value    interface{}
	Variable Variable
}
func (err InvalidVariableValue) Error() string {
	message := fmt.Sprintf("Value '%v' is not a valid value for variable '%s' with type '%s'.", err.Value, err.Variable.Name(), err.Variable.Type().String())
	if err.Variable.Type() == Enum {
		message = fmt.Sprintf("%s. Value must be one of: %s.", message, err.Variable.Options())
	}
	return message
}

type OptionsCanOnlyBeUsedWithEnum struct {
	Context string
	Type    BoilerplateType
}
func (err OptionsCanOnlyBeUsedWithEnum) Error() string {
	return fmt.Sprintf("%s has type %s and tries to specify options. Options may only be specified for the %s type.", err.Context, err.Type.String(), Enum)
}

type InvalidTypeForField struct {
	FieldName    string
	ExpectedType string
	ActualType   reflect.Type
	Context      string
}
func (err InvalidTypeForField) Error() string {
	message := fmt.Sprintf("Field %s in %s must have type %s but got %s", err.FieldName, err.Context, err.ExpectedType, err.ActualType)
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
	return fmt.Sprintf("Required field %s is missing", string(err))
}

type UnrecognizedBoilerplateType BoilerplateType
func (err UnrecognizedBoilerplateType) Error() string {
	return fmt.Sprintf("Unrecognized type: %s", BoilerplateType(err).String())
}