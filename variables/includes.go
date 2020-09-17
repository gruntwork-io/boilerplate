package variables

type Includes struct {
	BeforeIncludes []string
	AfterIncludes  []string
}

// Given a map of key:value pairs read from a Boilerplate YAML config file of the format:
//
// includes:
//   before:
//     - <PATH>
//     - <PATH>
//   after:
//     - <PATH>
//     - <PATH>
//
// This method takes the data above and unmarshals it into an Includes struct
func UnmarshalIncludesFromBoilerplateConfigYaml(fields map[string]interface{}) (Includes, error) {
	includeFields, err := unmarshalMapOfFields(fields, "includes")
	if err != nil {
		return Includes{}, err
	}

	beforeIncludes, err := unmarshalListOfStrings(includeFields, "before")
	if err != nil {
		return Includes{}, err
	}

	afterIncludes, err := unmarshalListOfStrings(includeFields, "after")
	if err != nil {
		return Includes{}, err
	}

	return Includes{BeforeIncludes: beforeIncludes, AfterIncludes: afterIncludes}, nil
}
