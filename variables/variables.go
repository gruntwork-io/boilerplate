package variables

import (
	"fmt"
	"github.com/gruntwork-io/boilerplate/util"
)

// An interface for a variable defined in a boilerplate.yml config file
type Variable interface {
	// The name of the variable
	Name() string

	// The full name of this variable, which includes its name and the dependency it is for (if any) in a
	// human-readable format
	FullName() string

	// The description of the variable, if any
	Description() string

	// The type of the variable
	Type() BoilerplateType

	// The default value for teh variable, if any
	Default() interface{}

	// The values this variable can take. Applies only if Type() is Enum.
	Options() []string

	// Return a copy of this variable but with the name set to the given name
	WithName(string) Variable

	// Return a copy of this variable but with the description set to the given description
	WithDescription(string) Variable

	// Return a copy of this variable but with the default set to the given value
	WithDefault(interface{}) Variable
}

// A private implementation of the Variable interface that forces all users to use our public constructors
type defaultVariable struct {
	name         string
	description  string
	defaultValue interface{}
	variableType BoilerplateType
	options      []string
}

func NewStringVariable(name string) Variable {
	return defaultVariable{
		name: name,
		variableType: String,
	}
}

func NewIntVariable(name string) Variable {
	return defaultVariable{
		name: name,
		variableType: Int,
	}
}

func NewFloatVariable(name string) Variable {
	return defaultVariable{
		name: name,
		variableType: Float,
	}
}

func NewBoolVariable(name string) Variable {
	return defaultVariable{
		name: name,
		variableType: Bool,
	}
}

func NewListVariable(name string, ) Variable {
	return defaultVariable{
		name: name,
		variableType: List,
	}
}

func NewMapVariable(name string) Variable {
	return defaultVariable{
		name: name,
		variableType: Map,
	}
}

func NewEnumVariable(name string, options []string) Variable {
	return defaultVariable{
		name: name,
		variableType: Enum,
		options: options,
	}
}

func (variable defaultVariable) Name() string {
	return variable.name
}

func (variable defaultVariable) FullName() string {
	dependencyName, variableName := SplitIntoDependencyNameAndVariableName(variable.Name())
	if dependencyName == "" {
		return variableName
	} else {
		return fmt.Sprintf("%s (for dependency %s)", variableName, dependencyName)
	}
}

func (variable defaultVariable) Description() string {
	return variable.description
}

func (variable defaultVariable) Type() BoilerplateType {
	return variable.variableType
}

func (variable defaultVariable) Default() interface{} {
	return variable.defaultValue
}

func (variable defaultVariable) Options() []string {
	return variable.options
}

func (variable defaultVariable) WithName(name string) Variable {
	variable.name = name
	return variable
}

func (variable defaultVariable) WithDescription(description string) Variable {
	variable.description = description
	return variable
}

func (variable defaultVariable) WithDefault(value interface{}) Variable {
	variable.defaultValue = value
	return variable
}

func UnmarshalValueForVariable(value interface{}, variable Variable) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch variable.Type() {
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
			for _, option := range variable.Options() {
				if asString == option {
					return asString, nil
				}
			}
		}
	}

	return nil, InvalidVariableValue{Variable: variable, Value: value}
}

func UnmarshalVariables(fields map[string]interface{}, fieldName string) ([]Variable, error) {
	unmarshalledVariables := []Variable{}

	listOfFields, err := unmarshalListOfFields(fields, fieldName)
	if err != nil {
		return unmarshalledVariables, err
	}

	for _, fields := range listOfFields {
		variable, err := UnmarshalVariable(fields)
		if err != nil {
			return unmarshalledVariables, err
		}
		unmarshalledVariables = append(unmarshalledVariables, variable)
	}

	return unmarshalledVariables, nil
}

// Given a map where the keys are the fields of a boilerplate Variable, this method crates a Variable struct with those
// fields filled in with proper types. This method also validates all the fields and returns an error if any problems
// are found.
func UnmarshalVariable(fields map[string]interface{}) (Variable, error) {
	variable := defaultVariable{}

	name, err := unmarshalStringField(fields, "name", true, "")
	if err != nil {
		return nil, err
	}
	variable.name = *name

	variableType, err := unmarshalTypeField(fields, "type", *name)
	if err != nil {
		return nil, err
	}
	variable.variableType = variableType

	description, err := unmarshalStringField(fields, "description", false, *name)
	if err != nil {
		return nil, err
	}
	if description != nil {
		variable.description = *description
	}

	options, err := unmarshalOptionsField(fields, "options", *name, variableType)
	if err != nil {
		return nil, err
	}
	variable.options = options

	defaultValue, err := UnmarshalValueForVariable(fields["default"], variable)
	if err != nil {
		return nil, err
	}
	variable.defaultValue = defaultValue

	return variable, nil
}
