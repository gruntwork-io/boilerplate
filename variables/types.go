package variables

import (
	"github.com/gruntwork-io/boilerplate/errors"
	"fmt"
)

// An enum that represents the types we support for boilerplate variables
type BoilerplateType string

var (
	String = BoilerplateType("string")
	Int = BoilerplateType("int")
	Float = BoilerplateType("float")
	Bool = BoilerplateType("bool")
	List = BoilerplateType("list")
	Map = BoilerplateType("map")
	Enum = BoilerplateType("enum")
)

var ALL_BOILERPLATE_TYPES = []BoilerplateType{String, Int, Float, Bool, List, Map, Enum}
var BOILERPLATE_TYPE_DEFAULT = String

// Convert the given string to a BoilerplateType enum, or return an error if this is not a valid value for the
// BoilerplateType enum
func ParseBoilerplateType(str string) (*BoilerplateType, error) {
	for _, boilerplateType := range ALL_BOILERPLATE_TYPES {
		if boilerplateType.String() == str {
			return &boilerplateType, nil
		}
	}
	return nil, errors.WithStackTrace(InvalidBoilerplateType(str))
}

// Return a string representation of this Type
func (boilerplateType BoilerplateType) String() string {
	return string(boilerplateType)
}

// Return an example value for this type. This is useful for showing a user the proper syntax to use for that type.
func (boilerplateType BoilerplateType) Example() string {
	switch boilerplateType {
	case String: return "foo"
	case Int: return "42"
	case Float: return "3.1415926"
	case Bool: return "true"
	case List: return "[foo, bar, baz]"
	case Map: return "{foo: bar, baz: blah}"
	case Enum: return "foo"
	default: return ""
	}
}

// Custom error types

type InvalidBoilerplateType string
func (err InvalidBoilerplateType) Error() string {
	return fmt.Sprintf("Invalid InvalidBoilerplateType '%s'. Value must be one of: %s", string(err), ALL_BOILERPLATE_TYPES)
}


