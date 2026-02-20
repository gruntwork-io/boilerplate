package variables

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/gruntwork-io/boilerplate/util"
	"gopkg.in/yaml.v2"
)

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//
//	key1: value1
//	key2: value2
//	key3: value3
//
// This method looks up the given fieldName in the map and unmarshals the data inside of it it into a map of the
// key:value pairs.
func unmarshalMapOfFields(fields map[string]any, fieldName string) (map[string]any, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	asYamlMap, isYamlMap := fieldAsYaml.(map[any]any)
	if !isYamlMap {
		return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "map[string]any", ActualType: reflect.TypeOf(fieldAsYaml)}
	}

	stringMap := map[string]any{}

	for key, value := range asYamlMap {
		if keyAsString, isString := key.(string); isString {
			stringMap[keyAsString] = value
		} else {
			return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(key)}
		}
	}

	return stringMap, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//
//	key1: value1
//	key2: value2
//	key3: value3
//
// This method looks up the given fieldName in the map and unmarshals the data inside of it it into a map of the
// key:value pairs, where both the keys and values are strings.
func unmarshalMapOfStrings(fields map[string]any, fieldName string) (map[string]string, error) {
	rawMap, err := unmarshalMapOfFields(fields, fieldName)
	if err != nil {
		return nil, err
	}

	if len(rawMap) == 0 {
		return nil, nil
	}

	stringMap := map[string]string{}

	for key, value := range rawMap {
		stringMap[key] = fmt.Sprintf("%v", value)
	}

	return stringMap, nil
}

// UnmarshalListOfStrings given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//   - value1
//   - value2
//   - value3
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it it into a list of
// strings with the given values.
func UnmarshalListOfStrings(fields map[string]any, fieldName string) ([]string, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	switch asList := fieldAsYaml.(type) {
	case []any:
		listOfStrings := []string{}

		for _, asYaml := range asList {
			if valueAsString, isString := asYaml.(string); isString {
				listOfStrings = append(listOfStrings, valueAsString)
			} else {
				return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeFor[[]any]()}
			}
		}

		return listOfStrings, nil
	case []string:
		return asList, nil
	default:
		return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "[]any or []string", ActualType: reflect.TypeOf(fieldAsYaml)}
	}
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName:
//
//   - key1: value1
//     key2: value2
//     key3: value3
//
//   - key1: value1
//     key2: value2
//     key3: value3
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it it into a list of
// maps, where each map contains the set of key:value pairs
func unmarshalListOfFields(fields map[string]any, fieldName string) ([]map[string]any, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	asYamlList, isYamlList := fieldAsYaml.([]any)
	if !isYamlList {
		return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "[]any", ActualType: reflect.TypeOf(fieldAsYaml)}
	}

	listOfFields := []map[string]any{}

	for _, asYaml := range asYamlList {
		asYamlMap, isYamlMap := asYaml.(map[any]any)
		if !isYamlMap {
			return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "map[string]any", ActualType: reflect.TypeOf(asYaml)}
		}

		listOfFields = append(listOfFields, util.ToStringToGenericMap(asYamlMap))
	}

	return listOfFields, nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// options:
//   - foo
//   - bar
//   - baz
//
// This method takes looks up the options object in the map and unmarshals the data inside of it it into a list of
// strings. This is meant to be used to parse the options field of an Enum variable. If the given variableType is not
// an Enum and options have been specified, or this variable is an Enum and options have not been specified, this
// method will return an error.
func unmarshalOptionsField(fields map[string]any, context string, variableType BoilerplateType) ([]string, error) {
	options, hasOptions := fields["options"]

	if !hasOptions {
		if variableType == Enum {
			return nil, OptionsMissing(context)
		} else {
			return nil, nil
		}
	}

	if variableType != Enum {
		return nil, OptionsCanOnlyBeUsedWithEnum{Context: context, Type: variableType}
	}

	optionsAsList, isList := options.([]any)
	if !isList {
		return nil, InvalidTypeForField{FieldName: "options", ExpectedType: "List", ActualType: reflect.TypeOf(options), Context: context}
	}

	return util.ToStringList(optionsAsList), nil
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// type: <TYPE>
//
// This method takes looks up the options key in the map and unmarshals the data inside of it it into a
// BoilerplateType. If no type is specified, this method returns the default type (String). If an unrecognized type is
// specified, this method returns an error.
func unmarshalTypeField(fields map[string]any, context string) (BoilerplateType, error) {
	variableTypeAsString, err := unmarshalStringField(fields, "type", false, context)
	if err != nil {
		return boilerplateTypeDefault, err
	}

	if variableTypeAsString != nil {
		variableType, err := ParseBoilerplateType(*variableTypeAsString)
		if err != nil {
			return boilerplateTypeDefault, err
		}

		return *variableType, nil
	}

	return boilerplateTypeDefault, nil
}

func unmarshalIntField(fields map[string]any, fieldName string, requiredField bool, context string) (*int, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return nil, RequiredFieldMissing(fieldName)
		} else {
			return nil, nil
		}
	}

	if valueAsInt, isInt := value.(int); isInt {
		return &valueAsInt, nil
	} else {
		return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "int", ActualType: reflect.TypeOf(value), Context: context}
	}
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName: <VALUE>
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it into a string. If
// requiredField is true and fieldName was not in the map, this method will return an error.
func unmarshalStringField(fields map[string]any, fieldName string, requiredField bool, context string) (*string, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return nil, RequiredFieldMissing(fieldName)
		} else {
			return nil, nil
		}
	}

	if valueAsString, isString := value.(string); isString {
		return &valueAsString, nil
	} else {
		return nil, InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(value), Context: context}
	}
}

// UnmarshalString is the public convenience interface for unmarshalStringField.
func UnmarshalString(fields map[string]any, fieldName string, isRequiredField bool) (*string, error) {
	return unmarshalStringField(fields, fieldName, isRequiredField, "")
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// fieldName: <VALUE>
//
// This method takes looks up the given fieldName in the map and unmarshals the data inside of it into a bool. If
// requiredField is true and fieldName was not in the map, this method will return an error.
func unmarshalBooleanField(fields map[string]any, fieldName string, requiredField bool, context string) (bool, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return false, RequiredFieldMissing(fieldName)
		} else {
			return false, nil
		}
	}

	if valueAsBool, isBool := value.(bool); isBool {
		return valueAsBool, nil
	} else {
		return false, InvalidTypeForField{FieldName: fieldName, ExpectedType: "bool", ActualType: reflect.TypeOf(value), Context: context}
	}
}

// parseVariablesFromEnvironmentVariables parses variables from environment variables
//
// These variables are expected to be in the format:
//
//	BOILERPLATE_var_name=value
func parseVariablesFromEnvironmentVariables() (map[string]any, error) {
	vars := map[string]any{}

	for _, envVar := range os.Environ() {
		if !strings.Contains(envVar, "BOILERPLATE_") {
			continue
		}

		key, value, found := strings.Cut(envVar, "=")
		if !found {
			return vars, InvalidVarSyntax(envVar)
		}

		if after, ok := strings.CutPrefix(key, "BOILERPLATE_"); ok {
			parsedValue, err := ParseYamlString(value)
			if err != nil {
				return vars, err
			}

			vars[after] = parsedValue
		}
	}

	return vars, nil
}

// Parse a list of NAME=VALUE pairs passed in as command-line options into a map of variable names to variable values.
// Along the way, each value is parsed as YAML.
func parseVariablesFromKeyValuePairs(varsList []string) (map[string]any, error) {
	vars := map[string]any{}

	for _, variable := range varsList {
		key, value, found := strings.Cut(variable, "=")
		if !found {
			return vars, InvalidVarSyntax(variable)
		}

		if key == "" {
			return vars, VariableNameCannotBeEmpty(variable)
		}

		parsedValue, err := ParseYamlString(value)
		if err != nil {
			return vars, err
		}

		vars[key] = parsedValue
	}

	return vars, nil
}

// ParseYamlString parses a YAML string into a Go type
func ParseYamlString(str string) (any, error) {
	var parsedValue any

	err := yaml.Unmarshal([]byte(str), &parsedValue)
	if err != nil {
		return nil, err
	}

	parsedValue, err = ConvertYAMLToStringMap(parsedValue)
	if err != nil {
		return nil, err
	}

	return parsedValue, nil
}

// Parse a list of YAML files that define variables into a map from variable name to variable value. Along the way,
// each value is parsed as YAML.
func parseVariablesFromVarFiles(varFileList []string) (map[string]any, error) {
	vars := map[string]any{}

	for _, varFile := range varFileList {
		varsInFile, err := ParseVariablesFromVarFile(varFile)
		if err != nil {
			return vars, err
		}

		vars = util.MergeMaps(vars, varsInFile)
	}

	return vars, nil
}

// ParseVariablesFromVarFile parses the variables in the given YAML file into a map of variable name to variable value. Along the way, each value
// is parsed as YAML.
func ParseVariablesFromVarFile(varFilePath string) (map[string]any, error) {
	bytes, err := os.ReadFile(varFilePath)
	if err != nil {
		return map[string]any{}, err
	}

	return parseVariablesFromVarFileContents(bytes)
}

// Parse the variables in the given YAML contents into a map of variable name to variable value. Along the way, each
// value is parsed as YAML.
func parseVariablesFromVarFileContents(varFileContents []byte) (map[string]any, error) {
	vars := make(map[string]any)

	if err := yaml.Unmarshal(varFileContents, &vars); err != nil {
		return vars, err
	}

	converted, err := ConvertYAMLToStringMap(vars)
	if err != nil {
		return nil, err
	}

	vars, ok := converted.(map[string]any)
	if !ok {
		return nil, YAMLConversionErr{converted}
	}

	return vars, nil
}

// ParseVars parses variables passed in via command line options, either as a list of NAME=VALUE variable pairs in varsList, or a
// list of paths to YAML files that define NAME: VALUE pairs. Return a map of the NAME: VALUE pairs. Along the way,
// each VALUE is parsed as YAML.
func ParseVars(varsList []string, varFileList []string) (map[string]any, error) {
	variables := map[string]any{}

	varsFromEnv, err := parseVariablesFromEnvironmentVariables()
	if err != nil {
		return variables, err
	}

	varsFromVarsList, err := parseVariablesFromKeyValuePairs(varsList)
	if err != nil {
		return variables, err
	}

	varsFromVarFiles, err := parseVariablesFromVarFiles(varFileList)
	if err != nil {
		return variables, err
	}

	return util.MergeMaps(varsFromEnv, varsFromVarsList, varsFromVarFiles), nil
}

// ConvertYAMLToStringMap modifies an input with type map[any]any to map[string]any so that it may be
// properly marshalled in to JSON.
// See: https://github.com/go-yaml/yaml/issues/139
func ConvertYAMLToStringMap(yamlMapOrList any) (any, error) {
	switch mapOrList := yamlMapOrList.(type) {
	case map[any]any:
		outputMap := map[string]any{}

		for k, v := range mapOrList {
			strK, ok := k.(string)
			if !ok {
				return nil, YAMLConversionErr{k}
			}

			res, err := ConvertYAMLToStringMap(v)
			if err != nil {
				return nil, err
			}

			outputMap[strK] = res
		}

		return outputMap, nil
	// catch cases where there is a map[any]any nested in a map[string]any
	case map[string]any:
		for k, v := range mapOrList {
			res, err := ConvertYAMLToStringMap(v)
			if err != nil {
				return nil, err
			}

			mapOrList[k] = res
		}

		return mapOrList, nil
	case []any:
		for index, value := range mapOrList {
			res, err := ConvertYAMLToStringMap(value)
			if err != nil {
				return nil, err
			}

			mapOrList[index] = res
		}
	}

	return yamlMapOrList, nil
}

// Custom error types

type YAMLConversionErr struct {
	Key any
}

func (err YAMLConversionErr) Error() string {
	return fmt.Sprintf("YAML value has type %s and cannot be cast to to the correct type.", reflect.TypeOf(err.Key))
}

type OptionsMissing string

func (err OptionsMissing) Error() string {
	return fmt.Sprintf("%s has type %s but does not specify any options. You must specify at least one option.", string(err), Enum)
}

type InvalidVariableValue struct {
	Value    any
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
	return "Variable name cannot be empty. Expected NAME=VALUE but got " + string(varSyntax)
}

type InvalidVarSyntax string

func (varSyntax InvalidVarSyntax) Error() string {
	return "Invalid syntax for variable. Expected NAME=VALUE but got " + string(varSyntax)
}

type RequiredFieldMissing string

func (err RequiredFieldMissing) Error() string {
	return fmt.Sprintf("Required field %s is missing", string(err))
}

type MutexRequiredFieldErr struct {
	fields []string
}

func (err MutexRequiredFieldErr) Error() string {
	return "Exactly one of the following fields must be set: " + strings.Join(err.fields, ",")
}

type UnrecognizedBoilerplateType BoilerplateType

func (err UnrecognizedBoilerplateType) Error() string {
	return "Unrecognized type: " + BoilerplateType(err).String()
}

type UndefinedOrderForFieldErr struct {
	fieldName string
}

func (err UndefinedOrderForFieldErr) Error() string {
	return "No order value defined for field: " + err.fieldName
}
