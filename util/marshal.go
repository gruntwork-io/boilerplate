package util

import (
	"fmt"

	"github.com/gruntwork-io/boilerplate/errors"
	"gopkg.in/yaml.v2"
)

// MarshalListOfObjectsToYAML will marshal the list of objects to yaml by calling MarshalYAML on every item in the list
// and return the results as a list. This is useful when building a custom YAML marshaler.
func MarshalListOfObjectsToYAML(inputList []any) ([]any, error) {
	output := []any{}

	for _, item := range inputList {
		itemAsMarshaler, hasType := item.(yaml.Marshaler)
		if !hasType {
			return nil, errors.WithStackTrace(UnmarshalableObjectErr{item})
		}

		yaml, err := itemAsMarshaler.MarshalYAML()
		if err != nil {
			return nil, errors.WithStackTrace(ObjectMarshalingErr{item, err})
		}

		output = append(output, yaml)
	}

	return output, nil
}

// Custom errors

// ObjectMarshalingErr is returned when there was an error marshaling the given object to yaml.
type ObjectMarshalingErr struct {
	object        any
	underlyingErr error
}

func (err ObjectMarshalingErr) Error() string {
	return fmt.Sprintf("Error marshaling %v to YAML: %s", err.object, err.underlyingErr)
}

// UnmarshalableObjectErr is returned when the given object does not implement Marshaler interface.
type UnmarshalableObjectErr struct {
	object any
}

func (err UnmarshalableObjectErr) Error() string {
	return fmt.Sprintf("Can not marshal %v to YAML", err.object)
}
