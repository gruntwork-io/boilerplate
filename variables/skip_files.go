package variables

// A single skip_file entry, which is a file that (conditionally) should be excluded from the rendered output.
type SkipFile struct {
	Path string
	If   string
}

// Implement the go-yaml marshaler interface so that the config can be marshaled into yaml. We use a custom marshaler
// instead of defining the fields as tags so that we skip the attributes that are empty.
func (skipFile SkipFile) MarshalYAML() (interface{}, error) {
	skipFileYml := map[string]interface{}{}
	if skipFile.Path != "" {
		skipFileYml["path"] = skipFile.Path
	}
	if skipFile.If != "" {
		skipFileYml["if"] = skipFile.If
	}
	return skipFileYml, nil
}

// Given a list of key:value pairs read from a Boilerplate YAML config file of the format:
//
// skip_files:
//   - path: <PATH>
//     if: <SKIPIF>
//   - path: <PATH>
//
// convert to a list of SkipFile structs.
func UnmarshalSkipFilesFromBoilerplateConfigYaml(fields map[string]interface{}) ([]SkipFile, error) {
	rawSkipFiles, err := unmarshalListOfFields(fields, "skip_files")
	if err != nil || rawSkipFiles == nil {
		return nil, err
	}

	skipFiles := []SkipFile{}

	for _, rawSkipFile := range rawSkipFiles {
		skipFile, err := unmarshalSkipFileFromBoilerplateConfigYaml(rawSkipFile)
		if err != nil {
			return nil, err
		}
		// We only return nil pointer when there is an error, so we can assume skipFile is non-nil at this point.
		skipFiles = append(skipFiles, *skipFile)
	}

	return skipFiles, nil
}

// Given key:value pairs read from a Boilerplate YAML config file of the format:
//
// path: <PATH>
// if: <SKIPIF>
//
// This method unmarshals the YAML data into a SkipFile struct
func unmarshalSkipFileFromBoilerplateConfigYaml(fields map[string]interface{}) (*SkipFile, error) {
	pathPtr, err := unmarshalStringField(fields, "path", true, "")
	if err != nil {
		return nil, err
	}

	// unmarshalStringField only returns nil pointer if there is an error, so we can assume it is not nil here.
	path := *pathPtr

	skipIfPtr, err := unmarshalStringField(fields, "if", false, path)
	if err != nil {
		return nil, err
	}
	skipIf := ""
	if skipIfPtr != nil {
		skipIf = *skipIfPtr
	}

	return &SkipFile{Path: path, If: skipIf}, nil
}
