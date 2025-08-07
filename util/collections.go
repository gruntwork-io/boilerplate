// Package util provides utility functions for common operations.
package util

import "fmt"

// MergeMaps merges all the maps into one. Sadly, Go has no generics, so this is only defined for string to interface maps.
func MergeMaps(maps ...map[string]any) map[string]any {
	out := map[string]any{}

	for _, currMap := range maps {
		for key, value := range currMap {
			out[key] = value
		}
	}

	return out
}

// ListContains returns true if the given list of strings (haystack) contains the given string (needle)
func ListContains(needle string, haystack []string) bool {
	for _, str := range haystack {
		if needle == str {
			return true
		}
	}

	return false
}

// ToStringList converts a generic list to a list of strings
func ToStringList(genericList []any) []string {
	stringList := []string{}

	for _, value := range genericList {
		stringList = append(stringList, ToString(value))
	}

	return stringList
}

// ToStringMap converts a generic map to a map from string to string
func ToStringMap(genericMap map[any]any) map[string]string {
	stringMap := map[string]string{}

	for key, value := range genericMap {
		stringMap[ToString(key)] = ToString(value)
	}

	return stringMap
}

// ToStringToGenericMap converts a generic map to a map from string to interface
func ToStringToGenericMap(genericMap map[any]any) map[string]any {
	stringToGenericMap := map[string]any{}

	for key, value := range genericMap {
		stringToGenericMap[ToString(key)] = value
	}

	return stringToGenericMap
}

// ToString converts a single value to its string representation
func ToString(value any) string {
	return fmt.Sprintf("%v", value)
}
