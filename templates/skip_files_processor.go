package templates

import (
	"fmt"
	"log/slog"
	"path/filepath"

	zglob "github.com/mattn/go-zglob"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/variables"
)

type ProcessedSkipFile struct {
	// List of paths relative to template folder that should be skipped
	EvaluatedPaths []string

	// List of paths relative to template folder that should not be skipped
	EvaluatedNotPaths []string

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
		matchedPaths, err := renderGlobPath(opts, skipFile.Path, variables)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}
		if skipFile.Path != "" {
			debugLogForMatchedPaths(skipFile.Path, matchedPaths, "SkipFile", "Path", opts.Logger)
		}

		matchedNotPaths, err := renderGlobPath(opts, skipFile.NotPath, variables)
		if err != nil {
			return nil, errors.WithStackTrace(err)
		}
		if skipFile.NotPath != "" {
			debugLogForMatchedPaths(skipFile.NotPath, matchedNotPaths, "SkipFile", "NotPath", opts.Logger)
		}

		renderedSkipIf, err := skipFileIfCondition(skipFile, opts, variables)
		if err != nil {
			return nil, err
		}

		processedSkipFile := ProcessedSkipFile{
			EvaluatedPaths:    matchedPaths,
			EvaluatedNotPaths: matchedNotPaths,
			RenderedSkipIf:    renderedSkipIf,
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

	rendered, err := render.RenderTemplateFromString(opts.TemplateFolder, skipFile.If, variables, opts)
	if err != nil {
		return false, err
	}

	// TODO: logger-debug - switch to debug
	if skipFile.Path != "" {
		opts.Logger.Info(fmt.Sprintf("If attribute for SkipFile Path %s evaluated to '%s'", skipFile.Path, rendered))
	} else if skipFile.NotPath != "" {
		opts.Logger.Info(fmt.Sprintf("If attribute for SkipFile NotPath %s evaluated to '%s'", skipFile.NotPath, rendered))
	} else {
		opts.Logger.Info(fmt.Sprintf("WARN: SkipFile has no path or not_path!"))
	}
	return rendered == "true", nil
}

func debugLogForMatchedPaths(sourcePath string, paths []string, directiveName string, directiveAttribute string, logger *slog.Logger) {
	// TODO: logger-debug - switch to debug
	logger.Info(fmt.Sprintf("Following paths were picked up by %s attribute for %s (%s):", directiveAttribute, directiveName, sourcePath))
	for _, path := range paths {
		logger.Info(fmt.Sprintf("\t- %s", path))
	}
}

// renderGlobPath will render the glob of the given path in the template folder and return the list of matched paths.
// Note that the paths will be canonicalized to unix slashes regardless of OS.
func renderGlobPath(opts *options.BoilerplateOptions, path string, variables map[string]interface{}) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}

	rendered, err := render.RenderTemplateFromString(opts.TemplateFolder, path, variables, opts)
	if err != nil {
		return nil, err
	}

	globPath := filepath.Join(opts.TemplateFolder, rendered)
	rawMatchedPaths, err := zglob.Glob(globPath)
	if err != nil {
		// TODO: logger-debug - switch to debug
		opts.Logger.Info(fmt.Sprintf("ERROR: could not glob %s", globPath))
		return nil, errors.WithStackTrace(err)
	}
	// Canonicalize the matched paths prior to storage
	matchedPaths := []string{}
	for _, path := range rawMatchedPaths {
		matchedPaths = append(matchedPaths, filepath.ToSlash(path))
	}
	return matchedPaths, nil
}
