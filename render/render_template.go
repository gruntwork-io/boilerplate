package render

import (
	"fmt"
	"bytes"
	"reflect"
	"text/template"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
)

const MaxRenderAttempts = 15

// Render the template at templatePath, with contents templateContents, using the Go template engine, passing in the
// given variables as data.
func RenderTemplate(templatePath string, templateContents string, variables map[string]interface{}, opts *options.BoilerplateOptions) (string, error) {
	option := fmt.Sprintf("missingkey=%s", string(opts.OnMissingKey))
	tmpl := template.New(templatePath).Funcs(CreateTemplateHelpers(templatePath, opts)).Option(option)

	parsedTemplate, err := tmpl.Parse(templateContents)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	var output bytes.Buffer
	if err := parsedTemplate.Execute(&output, variables); err != nil {
		return "", errors.WithStackTrace(err)
	}

	return output.String(), nil
}

// Render the template at templatePath, with contents templateContents, using the Go template engine, passing in the
// given variables as data. If the rendered result contains more Go templating syntax, render it again, and repeat this
// process recursively until there is no more rendering to be done.
//
// The main use case for this is to allow boilerplate variables to reference other boilerplate variables. This can
// obviously lead to an infinite loop. The proper way to prevent that would be to parse Go template syntax and build a
// dependency graph, but that is way too complicated. Therefore, we use hacky solution: render the template multiple
// times. If it is the same as the last time you rendered it, that means no new interpolations were processed, so
// we're done. If it changes, that means more interpolations are being processed, so keep going, up to a
// maximum number of render attempts.
func RenderTemplateRecursively(templatePath string, templateContents string, variables map[string]interface{}, opts *options.BoilerplateOptions) (string, error) {
	lastOutput := templateContents
	for i := 0; i < MaxRenderAttempts; i++ {
		output, err := RenderTemplate(templatePath, lastOutput, variables, opts)
		if err != nil {
			return "", err
		}

		if output == lastOutput {
			return output, nil
		}

		lastOutput = output
	}

	return "", errors.WithStackTrace(TemplateContainsInfiniteLoop{TemplatePath: templatePath, TemplateContents: templateContents, RenderAttempts: MaxRenderAttempts})
}


// Variable values are allowed to use Go templating syntax (e.g. to reference other variables), so this function loops
// over each variable value, renders each one, and returns a new map of rendered variables.
func RenderVariables(variables map[string]interface{}, opts *options.BoilerplateOptions) (map[string]interface{}, error) {
	renderedVariables := map[string]interface{}{}

	for variableName, variableValue := range variables {
		rendered, err := RenderVariable(variableValue, variables, opts)
		if err != nil {
			return nil, err
		}
		renderedVariables[variableName] = rendered
	}

	return renderedVariables, nil
}

// Variable values are allowed to use Go templating syntax (e.g. to reference other variables), so here, we render
// those templates and return a new map of variables that are fully resolved.
func RenderVariable(variable interface{}, variables map[string]interface{}, opts *options.BoilerplateOptions) (interface{}, error) {
	valueType := reflect.ValueOf(variable)

	switch valueType.Kind() {
	case reflect.String:
		return RenderTemplateRecursively(opts.TemplateFolder, variable.(string), variables, opts)
	case reflect.Slice:
		values := []interface{}{}
		for i := 0; i < valueType.Len(); i++ {
			rendered, err := RenderVariable(valueType.Index(i).Interface(), variables, opts)
			if err != nil {
				return  nil, err
			}
			values = append(values, rendered)
		}
		return values, nil
	case reflect.Map:
		values := map[interface{}]interface{}{}
		for _, key := range valueType.MapKeys() {
			renderedKey, err := RenderVariable(key.Interface(), variables, opts)
			if err != nil {
				return nil, err
			}
			renderedValue, err := RenderVariable(valueType.MapIndex(key).Interface(), variables, opts)
			if err != nil {
				return nil, err
			}
			values[renderedKey] = renderedValue
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