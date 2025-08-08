package variables

import "github.com/gruntwork-io/boilerplate/errors"

// SkipFile represents a single skip_file entry, which is a file that (conditionally) should be excluded from the rendered output.
type SkipFile struct {
	Path    string
	NotPath string
	If      string
}

// MarshalYAML implements the go-yaml marshaler interface so that the config can be marshaled into yaml. We use a custom marshaler
// instead of defining the fields as tags so that we skip the attributes that are empty.
func (skipFile SkipFile) MarshalYAML() (any, error) {
	skipFileYml := map[string]any{}
	if skipFile.Path != "" {
		skipFileYml["path"] = skipFile.Path
	}

	if skipFile.NotPath != "" {
		skipFileYml["not_path"] = skipFile.Path
	}

	if skipFile.If != "" {
		skipFileYml["if"] = skipFile.If
	}

	return skipFileYml, nil
}

// UnmarshalSkipFilesFromBoilerplateConfigYaml given a list of key:value pairs read from a Boilerplate YAML config file of the format:
//
// skip_files:
//   - path: <PATH>
//     if: <SKIPIF>
//   - path: <PATH>
//   - not_path: <PATH>
//
// convert to a list of SkipFile structs.
func UnmarshalSkipFilesFromBoilerplateConfigYaml(fields map[string]any) ([]SkipFile, error) {
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
// not_path: <PATH>
// if: <SKIPIF>
//
// This method unmarshals the YAML data into a SkipFile struct
func unmarshalSkipFileFromBoilerplateConfigYaml(fields map[string]any) (*SkipFile, error) {
	pathPtr, err := unmarshalStringField(fields, "path", false, "")
	if err != nil {
		return nil, err
	}

	path := ""
	if pathPtr != nil {
		path = *pathPtr
	}

	notPathPtr, err := unmarshalStringField(fields, "not_path", false, "")
	if err != nil {
		return nil, err
	}

	notPath := ""
	if notPathPtr != nil {
		notPath = *notPathPtr
	}

	// One of not_path or path must be set, so we check that here.
	if (notPath == "" && path == "") || (notPath != "" && path != "") {
		return nil, errors.WithStackTrace(MutexRequiredFieldErr{fields: []string{"path", "not_path"}})
	}

	skipIfPtr, err := unmarshalStringField(fields, "if", false, path)
	if err != nil {
		return nil, err
	}

	skipIf := ""
	if skipIfPtr != nil {
		skipIf = *skipIfPtr
	}

	return &SkipFile{Path: path, NotPath: notPath, If: skipIf}, nil
}
