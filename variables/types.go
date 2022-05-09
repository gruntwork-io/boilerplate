package variables

import (
	"fmt"

	"github.com/gruntwork-io/boilerplate/errors"
)

// An enum that represents the types we support for boilerplate variables
type BoilerplateType string

var (
	String = BoilerplateType("string")
	Int    = BoilerplateType("int")
	Float  = BoilerplateType("float")
	Bool   = BoilerplateType("bool")
	List   = BoilerplateType("list")
	Map    = BoilerplateType("map")
	Enum   = BoilerplateType("enum")
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

type InvalidEntries struct {
	Issues []ValidationIssue
}

type ValidationIssue struct {
	Value         interface{}
	ValidationMap map[string]bool
}

// Custom error types

type InvalidBoilerplateType string

func (err InvalidBoilerplateType) Error() string {
	return fmt.Sprintf("Invalid InvalidBoilerplateType '%s'. Value must be one of: %s", string(err), ALL_BOILERPLATE_TYPES)
}
