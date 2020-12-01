package templates

import (
	"path/filepath"

	zglob "github.com/mattn/go-zglob"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

type ProcessedSkipFile struct {
	// List of paths relative to template folder that should be skipped
	EvaluatedPaths []string

	// Whether or not to skip the files if the paths match. This is the boilerplate rendered value of the if attribute
	// of a given skip file.
	RenderedSkipIf bool
}

// processSkipFiles will take the skip_files list and process them in the current boilerplate context. This includes:
// - Rendering the glob expression for the Path attribute.
// - Rendering the if attribute using the provided variables.
func processSkipFiles(skipFiles []variables.SkipFile, opts *options.BoilerplateOptions, variables map[string]interface{}) ([]ProcessedSkipFile, error) {
	output := []ProcessedSkipFile{}
	for _, skipFile := range skipFiles {
		matchedPaths, err := renderGlobPath(opts, skipFile.Path, "SkipFile")
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}

		renderedSkipIf, err := skipFileIfCondition(skipFile, opts, variables)
		if err != nil {
			return nil, err
		}

		processedSkipFile := ProcessedSkipFile{
			EvaluatedPaths: matchedPaths,
			RenderedSkipIf: renderedSkipIf,
		}
		output = append(output, processedSkipFile)
	}
	return output, nil
}

// Return true if the if parameter of the given SkipFile evaluates to a "true" value
func skipFileIfCondition(skipFile variables.SkipFile, opts *options.BoilerplateOptions, variables map[string]interface{}) (bool, error) {
	// If the "if" attribute of skip_files was not specified, then default to true.
	if skipFile.If == "" {
		return true, nil
	}

	rendered, err := render.RenderTemplateRecursively(opts.TemplateFolder, skipFile.If, variables, opts)
	if err != nil {
		return false, err
	}

	// TODO: logger-debug - switch to debug
	util.Logger.Printf("If attribute for SkipFile Path %s evaluated to '%s'", skipFile.Path, rendered)
	return rendered == "true", nil
}

func debugLogForMatchedPaths(sourcePath string, paths []string, directiveName string) {
	// TODO: logger-debug - switch to debug
	util.Logger.Printf("Following paths were picked up by Path attribute for %s (%s):", directiveName, sourcePath)
	for _, path := range paths {
		util.Logger.Printf("\t- %s", path)
	}
}

// renderGlobPath will render the glob of the given path in the template folder and return the list of matched paths.
// Note that the paths will be canonicalized to unix slashes regardless of OS.
func renderGlobPath(opts *options.BoilerplateOptions, path string, directiveName string) ([]string, error) {
	rawMatchedPaths, err := zglob.Glob(filepath.Join(opts.TemplateFolder, path))
	if err != nil {
		return nil, errors.WithStackTrace(err)
	}
	// Canonicalize the matched paths prior to storage
	matchedPaths := []string{}
	for _, path := range rawMatchedPaths {
		matchedPaths = append(matchedPaths, filepath.ToSlash(path))
	}
	debugLogForMatchedPaths(path, matchedPaths, directiveName)
	return matchedPaths, nil
}
