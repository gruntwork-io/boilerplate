package render

import (
	"bytes"
	"fmt"
	"path"
	"reflect"
	"text/template"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
)

const MaxRenderAttempts = 15

// RenderTemplateWithPartials renders the template at templatePath with the contents of the root template (the template
// named by the user on the command line) as well as all of the partials matched by the provided globs using the Go
// template engine, passing in the given variables as data.
func RenderTemplateWithPartials(templatePath string, partials []string, variables map[string]interface{}, opts *options.BoilerplateOptions) (string, error) {
	tmpl, err := getTemplate(templatePath, opts).ParseGlob(templatePath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	// Each item in the list of partials is a glob to a path relative to the templatePath, so we need to
	// first resolve the path, then parse all the files matching the glob. Finally, we add all the templates
	// found in each glob to the tree.
	for _, globOfPartials := range partials {
		// Use opts.TemplateFolder because the templatePath may be a subdir, but the partial paths are
		// relative to the path passed in by the user
		relativePath := PathRelativeToTemplate(opts.TemplateFolder, globOfPartials)
		parsedTemplate, err := getTemplate(templatePath, opts).ParseGlob(relativePath)
		if err != nil {
			return "", errors.WithStackTrace(err)
		}
		for _, t := range parsedTemplate.Templates() {
			tmpl.AddParseTree(t.Name(), t.Tree)
		}
	}
	return executeTemplate(tmpl, variables)
}

// Render the template at templatePath, with contents templateContents, using the Go template engine, passing in the
// given variables as data.
func RenderTemplateFromString(templatePath string, templateContents string, variables map[string]interface{}, opts *options.BoilerplateOptions) (string, error) {
	tmpl := getTemplate(templatePath, opts)
	parsedTemplate, err := tmpl.Parse(templateContents)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return executeTemplate(parsedTemplate, variables)
}

// getTemplate returns new template initialized with options and helper functions
func getTemplate(templatePath string, opts *options.BoilerplateOptions) *template.Template {
	tmpl := template.New(path.Base(templatePath))
	option := fmt.Sprintf("missingkey=%s", string(opts.OnMissingKey))
	return tmpl.Funcs(CreateTemplateHelpers(templatePath, opts, tmpl)).Option(option)
}

// executeTemplate executes a parsed template with a given set of variable inputs and return the output as a string
func executeTemplate(tmpl *template.Template, variables map[string]interface{}) (string, error) {
	var output bytes.Buffer
	if err := tmpl.Execute(&output, variables); err != nil {
		return "", errors.WithStackTrace(err)
	}
	return output.String(), nil
}

// Variable values are allowed to use Go templating syntax (e.g. to reference other variables), so this function loops
// over each variable value, renders each one, and returns a new map of rendered variables.
func RenderVariables(opts *options.BoilerplateOptions, variables map[string]interface{}) (map[string]interface{}, error) {
	// TODO: explain
	optsForRender := *opts
	optsForRender.OnMissingKey = options.ExitWithError

	unrenderedVariables := []string{}
	for variableName, _ := range variables {
		unrenderedVariables = append(unrenderedVariables, variableName)
	}

	var renderErr error
	renderedVariables := map[string]interface{}{}
	rendered := true
	for iterations := 0; len(unrenderedVariables) > 0 && rendered; iterations++ {
		if iterations > MaxRenderAttempts {
			// Reached maximum supported iterations, which is most likely an infinite loop bug so cut the iteration
			// short an return an error.
			return nil, fmt.Errorf("TODO: concrete error")
		}

		unrenderedVariables, renderedVariables, rendered, renderErr = attemptRenderVariables(&optsForRender, unrenderedVariables, renderedVariables, variables)
	}
	if len(unrenderedVariables) > 0 {
		return nil, renderErr
	}

	return renderedVariables, nil
}

func attemptRenderVariables(
	opts *options.BoilerplateOptions,
	unrenderedVariables []string,
	renderedVariables map[string]interface{},
	variables map[string]interface{},
) ([]string, map[string]interface{}, bool, error) {
	newUnrenderedVariables := []string{}
	wasRendered := false

	// TODO: collect render errs
	for _, variableName := range unrenderedVariables {
		rendered, err := attemptRenderVariable(opts, variables[variableName], renderedVariables)
		if err != nil {
			newUnrenderedVariables = append(newUnrenderedVariables, variableName)
		} else {
			renderedVariables[variableName] = rendered
			wasRendered = true
		}
	}
	return newUnrenderedVariables, renderedVariables, wasRendered, nil
}

// Variable values are allowed to use Go templating syntax (e.g. to reference other variables), so here, we render
// those templates and return a new map of variables that are fully resolved.
func attemptRenderVariable(opts *options.BoilerplateOptions, variable interface{}, renderedVariables map[string]interface{}) (interface{}, error) {
	valueType := reflect.ValueOf(variable)

	switch valueType.Kind() {
	case reflect.String:
		return RenderTemplateFromString(opts.TemplateFolder, variable.(string), renderedVariables, opts)
	case reflect.Slice:
		values := []interface{}{}
		for i := 0; i < valueType.Len(); i++ {
			rendered, err := attemptRenderVariable(opts, valueType.Index(i).Interface(), renderedVariables)
			if err != nil {
				return nil, err
			}
			values = append(values, rendered)
		}
		return values, nil
	case reflect.Map:
		values := map[string]interface{}{}
		for _, key := range valueType.MapKeys() {
			renderedKey, err := attemptRenderVariable(opts, key.Interface(), renderedVariables)
			if err != nil {
				return nil, err
			}
			renderedValue, err := attemptRenderVariable(opts, valueType.MapIndex(key).Interface(), renderedVariables)
			if err != nil {
				return nil, err
			}
			values[renderedKey.(string)] = renderedValue
		}
		return values, nil
	default:
		return variable, nil
	}
}

// Custom error types

type TemplateContainsInfiniteLoop struct {
	TemplatePath     string
	TemplateContents string
	RenderAttempts   int
}

func (err TemplateContainsInfiniteLoop) Error() string {
	return fmt.Sprintf("Template %s seems to contain infinite loop. After %d renderings, the contents continue to change. Template contents:\n%s", err.TemplatePath, err.RenderAttempts, err.TemplateContents)
}
