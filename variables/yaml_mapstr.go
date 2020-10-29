// Copyright (c) 2015-2016 Michael Persson
// Copyright (c) 2012–2015 Elasticsearch <http://www.elastic.co>
//
// Originally distributed as part of "beats" repository (https://github.com/elastic/beats).
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
)

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
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
