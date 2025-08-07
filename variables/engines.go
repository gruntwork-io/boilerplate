package variables

import (
	"fmt"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/util"
)

// Engine represents a single engine entry, which specifies which template engine should be used to render the given files grabbed by the
// glob. Currently only the following template engines are supported:
//
// - Go template (default)
// - Jsonnet
type Engine struct {
	Path           string             `yaml:"path"`
	TemplateEngine TemplateEngineType `yaml:"template_engine"`
}

type TemplateEngineType string

const (
	GoTemplate            TemplateEngineType = "go-template"
	Jsonnet               TemplateEngineType = "jsonnet"
	DefaultTemplateEngine                    = GoTemplate
)

// availableTemplateEngines is a list of string representations of the TemplateEngineType enum. This is used for
// validating user input.
var availableTemplateEngines = []string{
	string(GoTemplate),
	string(Jsonnet),
}

// UnmarshalEnginesFromBoilerplateConfigYaml given a list of key:value pairs read from a Boilerplate YAML config file of the format:
//
// engines:
//   - path: <PATH>
//     template_engine: <TEMPLATE_ENGINE>
//
// convert to a list of Engine structs.
func UnmarshalEnginesFromBoilerplateConfigYaml(fields map[string]interface{}) ([]Engine, error) {
	rawEngines, err := unmarshalListOfFields(fields, "engines")
	if err != nil || rawEngines == nil {
		return nil, err
	}

	engines := []Engine{}

	for _, rawEngine := range rawEngines {
		engine, err := unmarshalEngineFromBoilerplateConfigYaml(rawEngine)
		if err != nil {
			return nil, err
		}
		// We only return nil pointer when there is an error, so we can assume engine is non-nil at this point.
		engines = append(engines, *engine)
	}

	return engines, nil
}

// Given key:value pairs read from a Boilerplate YAML config file of the format:
//
// path: <PATH>
// template_engine: <TEMPLATE_ENGINE>
//
// This method unmarshals the YAML data into an Engine struct
func unmarshalEngineFromBoilerplateConfigYaml(fields map[string]interface{}) (*Engine, error) {
	pathPtr, err := unmarshalStringField(fields, "path", true, "")
	if err != nil {
		return nil, err
	}

	// unmarshalStringField only returns nil pointer if there is an error, so we can assume it is not nil here.
	path := *pathPtr

	templateEnginePtr, err := unmarshalStringField(fields, "template_engine", true, path)
	if err != nil {
		return nil, err
	}

	// unmarshalStringField only returns nil pointer if there is an error, so we can assume it is not nil here.
	maybeTemplateEngine := *templateEnginePtr

	// Validate the template engine conforms to enum.
	if !util.ListContains(maybeTemplateEngine, availableTemplateEngines) {
		return nil, errors.WithStackTrace(InvalidTemplateEngineErr(maybeTemplateEngine))
	}

	return &Engine{Path: path, TemplateEngine: TemplateEngineType(maybeTemplateEngine)}, nil
}

// InvalidTemplateEngineErr represents custom errors
type InvalidTemplateEngineErr string

func (err InvalidTemplateEngineErr) Error() string {
	return fmt.Sprintf("%s is not a valid template engine. Must be one of %v", string(err), availableTemplateEngines)
}
