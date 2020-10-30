// Copyright (c) 2015-2016 Michael Persson
// Copyright (c) 2012–2015 Elasticsearch <http://www.elastic.co>
//
// Originally distributed as part of "beats" repository (https://github.com/elastic/beats).
//
// Modified for use with Gruntwork Boilerplate
//
// Distributed underneath "Apache License, Version 2.0" which is compatible with the LICENSE for this package.
//
// This is included as a workaround for the following issue in the go-yaml package:
//   https://github.com/go-yaml/yaml/issues/139
// Based off the code here:
//    https://github.com/go-yaml/yaml/issues/139#issuecomment-220072190

package variables

import (
	// Base packages.
	"fmt"

	// Third party packages.
	"gopkg.in/yaml.v2"
)

// Unmarshal YAML to map[string]interface{} instead of map[interface{}]interface{}.
func UnmarshalYaml(in []byte, out interface{}) error {
	res := make(map[string]interface{})

	if err := yaml.Unmarshal(in, &res); err != nil {
		return err
	}

	for k, v := range res {
		res[k] = cleanupMapValue(v)
	}
	*out.(*map[string]interface{}) = res

	return nil
}

// Marshal YAML wrapper function.
func Marshal(in interface{}) ([]byte, error) {
	return yaml.Marshal(in)
}

func cleanupInterfaceArray(in []interface{}) []interface{} {
	res := make([]interface{}, len(in))
	for i, v := range in {
		res[i] = cleanupMapValue(v)
	}
	return res
}

func cleanupInterfaceMap(in map[interface{}]interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range in {
		res[fmt.Sprintf("%v", k)] = cleanupMapValue(v)
	}
	return res
}

func cleanupMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return cleanupInterfaceArray(v)
	case map[interface{}]interface{}:
		return cleanupInterfaceMap(v)
	default:
		return v
	}
}
