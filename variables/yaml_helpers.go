package variables

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/gruntwork-io/boilerplate/errors"
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
func unmarshalMapOfFields(fields map[string]any, fieldName string) (map[string]interface{}, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	asYamlMap, isYamlMap := fieldAsYaml.(map[any]interface{})
	if !isYamlMap {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "map[string]any", ActualType: reflect.TypeOf(fieldAsYaml)})
	}

	stringMap := map[string]any{}
	for key, value := range asYamlMap {
		if keyAsString, isString := key.(string); isString {
			stringMap[keyAsString] = value
		} else {
			return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(key)})
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

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
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
				return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "string", ActualType: reflect.TypeOf(asList)})
			}
		}

		return listOfStrings, nil
	case []string:
		return asList, nil
	default:
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "[]any or []string", ActualType: reflect.TypeOf(fieldAsYaml)})
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
func unmarshalListOfFields(fields map[string]any, fieldName string) ([]map[string]interface{}, error) {
	fieldAsYaml, containsField := fields[fieldName]
	if !containsField || fieldAsYaml == nil {
		return nil, nil
	}

	asYamlList, isYamlList := fieldAsYaml.([]any)
	if !isYamlList {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "[]any", ActualType: reflect.TypeOf(fieldAsYaml)})
	}

	listOfFields := []map[string]any{}

	for _, asYaml := range asYamlList {
		asYamlMap, isYamlMap := asYaml.(map[any]interface{})
		if !isYamlMap {
			return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "map[string]any", ActualType: reflect.TypeOf(asYaml)})
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
			return nil, errors.WithStackTrace(OptionsMissing(context))
		} else {
			return nil, nil
		}
	}

	if variableType != Enum {
		return nil, errors.WithStackTrace(OptionsCanOnlyBeUsedWithEnum{Context: context, Type: variableType})
	}

	optionsAsList, isList := options.([]any)
	if !isList {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: "options", ExpectedType: "List", ActualType: reflect.TypeOf(options), Context: context})
	}

	return util.ToStringList(optionsAsList), nil
}

type CustomValidationRule struct {
	Validator validation.Rule
	Message   string
}

type CustomValidationRuleCollection []CustomValidationRule

func (c CustomValidationRuleCollection) GetValidators() []validation.Rule {
	var validatorsToReturn []validation.Rule
	for _, rule := range c {
		validatorsToReturn = append(validatorsToReturn, rule.Validator)
	}
	return validatorsToReturn
}

func (c CustomValidationRule) DescriptionText() string {
	return c.Message
}

// parseRuleString converts the string representation of the validations field, as parsed from YAML,
// into a slice of strings that ConvertValidationStringtoRules can easily iterate over
func parseRuleString(ruleString string) []string {
	ruleString = strings.ReplaceAll(ruleString, "]", "")
	ruleString = strings.ReplaceAll(ruleString, "[", "")
	ruleString = strings.ToLower(ruleString)
	return strings.Split(ruleString, " ")
}

// ConvertValidationStringtoRules takes the string representation of the variable's validations and parses it
// into CustomValidationRules that should be run on the variable's value when submitted by a user
func ConvertValidationStringtoRules(ruleString string) ([]CustomValidationRule, error) {
	var validationRules []CustomValidationRule

	rules := parseRuleString(ruleString)

	for _, rule := range rules {

		var cvr CustomValidationRule

		switch {
		case strings.HasPrefix(rule, "length-"):
			valString := strings.TrimLeft(rule, "length-")
			valSlice := strings.Split(valString, "-")
			minStr := valSlice[0]
			maxStr := valSlice[1]
			min, minErr := strconv.Atoi(strings.TrimSpace(minStr))
			if minErr != nil {
				return validationRules, minErr
			}
			max, maxErr := strconv.Atoi(strings.TrimSpace(maxStr))
			if maxErr != nil {
				return validationRules, maxErr
			}
			cvr = CustomValidationRule{
				Validator: validation.Length(min, max),
				Message:   fmt.Sprintf("Must be between %d and %d characters long", min, max),
			}
		case rule == "required":
			cvr = CustomValidationRule{
				Validator: validation.Required,
				Message:   "Must not be empty",
			}
		case rule == "url":
			cvr = CustomValidationRule{
				Validator: is.URL,
				Message:   "Must be a valid URL",
			}
		case rule == "email":
			cvr = CustomValidationRule{
				Validator: is.Email,
				Message:   "Must be a valid email address",
			}
		case rule == "alpha":
			cvr = CustomValidationRule{
				Validator: is.Alpha,
				Message:   "Must contain English letters only",
			}
		case rule == "digit":
			cvr = CustomValidationRule{
				Validator: is.Digit,
				Message:   "Must contain digits only",
			}
		case rule == "alphanumeric":
			cvr = CustomValidationRule{
				Validator: is.Alphanumeric,
				Message:   "Can contain English letters and digits only",
			}
		case rule == "countrycode2":
			cvr = CustomValidationRule{
				Validator: is.CountryCode2,
				Message:   "Must be a valid ISO3166 Alpha 2 Country code",
			}
		case rule == "semver":
			cvr = CustomValidationRule{
				Validator: is.Semver,
				Message:   "Must be a valid semantic version",
			}
		}

		if cvr != (CustomValidationRule{}) {
			validationRules = append(validationRules, cvr)
		}
	}

	return validationRules, nil
}

// Given a list of validations read from a Boilerplate YAML config file of the format:
//
// This method looks up the validations specified in the map and applies them to the specified fields so that users prompted for input
// get real-time feedback on the validity of their entries
func unmarshalValidationsField(fields map[string]any) ([]CustomValidationRule, error) {
	validations := fields["validations"]

	validationsAsString := fmt.Sprintf("%v", validations)

	rules, err := ConvertValidationStringtoRules(validationsAsString)
	if err != nil {
		return []CustomValidationRule{}, err
	}

	return rules, nil
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

func unmarshalIntField(fields map[string]any, fieldName string, requiredField bool, context string) (*int, error) {
	value, hasValue := fields[fieldName]
	if !hasValue {
		if requiredField {
			return nil, errors.WithStackTrace(RequiredFieldMissing(fieldName))
		} else {
			return nil, nil
		}
	}

	if valueAsInt, isInt := value.(int); isInt {
		return &valueAsInt, nil
	} else {
		return nil, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "int", ActualType: reflect.TypeOf(value), Context: context})
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
			return false, errors.WithStackTrace(RequiredFieldMissing(fieldName))
		} else {
			return false, nil
		}
	}

	if valueAsBool, isBool := value.(bool); isBool {
		return valueAsBool, nil
	} else {
		return false, errors.WithStackTrace(InvalidTypeForField{FieldName: fieldName, ExpectedType: "bool", ActualType: reflect.TypeOf(value), Context: context})
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
			return vars, errors.WithStackTrace(InvalidVarSyntax(envVar))
		}

		if strings.HasPrefix(key, "BOILERPLATE_") {
			key = strings.TrimPrefix(key, "BOILERPLATE_")
			parsedValue, err := ParseYamlString(value)
			if err != nil {
				return vars, err
			}

			vars[key] = parsedValue
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
			return vars, errors.WithStackTrace(InvalidVarSyntax(variable))
		}

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
func ParseYamlString(str string) (any, error) {
	var parsedValue any

	err := yaml.Unmarshal([]byte(str), &parsedValue)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	parsedValue, err = ConvertYAMLToStringMap(parsedValue)
	if err != nil {
		return nil, errors.WithStackTrace(err)
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

// Parse the variables in the given YAML or JSON file into a map of variable name to variable value. 
// The parsing format is determined by the file extension (.json for JSON, everything else for YAML).
func ParseVariablesFromVarFile(varFilePath string) (map[string]any, error) {
	bytes, err := os.ReadFile(varFilePath)
	if err != nil {
		return map[string]any{}, errors.WithStackTrace(err)
	}

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(varFilePath))
	if ext == ".json" {
		return parseVariablesFromJsonFileContents(bytes)
	} else {
		// Default to YAML for .yml, .yaml, or any other extension
		return parseVariablesFromVarFileContents(bytes)
	}
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
		return nil, errors.WithStackTrace(err)
	}

	vars, ok := converted.(map[string]any)
	if !ok {
		return nil, YAMLConversionErr{converted}
	}

	return vars, nil
}

// Parse the variables in the given JSON contents into a map of variable name to variable value.
func parseVariablesFromJsonFileContents(jsonFileContents []byte) (map[string]any, error) {
	vars := make(map[string]any)

	if err := json.Unmarshal(jsonFileContents, &vars); err != nil {
		return vars, errors.WithStackTrace(err)
	}

	converted, err := ConvertYAMLToStringMap(vars)
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}

	vars, ok := converted.(map[string]any)
	if !ok {
		return nil, YAMLConversionErr{converted}
	}

	return vars, nil
}

// Parse variables passed in via command line options, either as a list of NAME=VALUE variable pairs in varsList, or a
// list of paths to YAML or JSON files that define NAME: VALUE pairs. Return a map of the NAME: VALUE pairs. Along the way,
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

// convertYAMLToStringMap modifies an input with type map[any]interface{} to map[string]interface{} so that it may be
// properly marshalled in to JSON.
// See: https://github.com/go-yaml/yaml/issues/139
func ConvertYAMLToStringMap(yamlMapOrList any) (interface{}, error) {
	switch mapOrList := yamlMapOrList.(type) {
	case map[any]interface{}:
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

type ValidationsMissing string

func (err ValidationsMissing) Error() string {
	return fmt.Sprintf("%s does not specify any validations. You must specify at least one validation.", string(err))
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

type MutexRequiredFieldErr struct {
	fields []string
}

func (err MutexRequiredFieldErr) Error() string {
	return fmt.Sprintf("Exactly one of the following fields must be set: %s", strings.Join(err.fields, ","))
}

type UnrecognizedBoilerplateType BoilerplateType

func (err UnrecognizedBoilerplateType) Error() string {
	return fmt.Sprintf("Unrecognized type: %s", BoilerplateType(err).String())
}

type UndefinedOrderForFieldErr struct {
	fieldName string
}

func (err UndefinedOrderForFieldErr) Error() string {
	return fmt.Sprintf("No order value defined for field: %s", err.fieldName)
}
