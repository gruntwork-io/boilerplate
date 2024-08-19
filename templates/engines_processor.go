package templates

import (
	"path/filepath"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

type ProcessedEngine struct {
	// List of paths relative to template folder that should be skipped
	EvaluatedPaths []string

	// The template engine to use.
	TemplateEngine variables.TemplateEngineType
}

// processEngines will take the engines list and process them in the current boilerplate context. This is primarily
// rendering the glob expression for the Path attribute.
func processEngines(
	engines []variables.Engine,
	opts *options.BoilerplateOptions,
	variables map[string]interface{},
) ([]ProcessedEngine, error) {
	output := []ProcessedEngine{}
	for _, engine := range engines {
		matchedPaths, err := renderGlobPath(opts, engine.Path, variables)
		if err != nil {
			return nil, err
		}
		debugLogForMatchedPaths(engine.Path, matchedPaths, "Engine", "Path", opts.Logger)

		processedEngine := ProcessedEngine{
			EvaluatedPaths: matchedPaths,
			TemplateEngine: engine.TemplateEngine,
		}
		output = append(output, processedEngine)
	}
	return output, nil
}

// determineTemplateEngine returns the template engine that should be used based on the engine directive and the path of
// the template file to process.
func determineTemplateEngine(processedEngines []ProcessedEngine, path string) variables.TemplateEngineType {
	// Canonicalize paths for os portability.
	canonicalPath := filepath.ToSlash(path)

	// If the path matches any of the engine directives, return the engine specified by that directive. Otherwise,
	// return the default template engine.
	for _, engine := range processedEngines {
		if util.ListContains(canonicalPath, engine.EvaluatedPaths) {
			return engine.TemplateEngine
		}
	}
	return variables.DefaultTemplateEngine

}
