package render

import (
	"bytes"
	"fmt"
	"path"
	"reflect"
	"text/template"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/hashicorp/go-multierror"
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

// RenderVariables will render each of the variables that need to be rendered by running it through the go templating
// syntax. Variable values are allowed to use Go templating syntax (e.g. to reference other variables), so this function
// loops over each variable value, renders each one, and returns a new map of rendered variables.
//
// This function supports nested variables references, but uses a heuristic based approach. Ideally, we can parse the Go
// template and build up a graph of variable dependencies to assist with the rendering process, but this takes a lot of
// effort to get right and maintain.
//
// Instead, we opt for a simpler approach of rendering with multiple trials. In this approach, we continuously attempt
// to render the template on the variable until all of them render without errors, or we reach the maximum trials. To
// support this, we ignore the missing key configuration during this evaluation pass and always rely on the template
// erroring for missing variables. Otherwise, all the variables will render on the first pass.
//
// Note that this is NOT a multi pass algorithm - that is, we do NOT attempt to render the template multiple times.
// Instead, we do a single template render on each run and reject any that return with an error.
func RenderVariables(
	opts *options.BoilerplateOptions,
	variablesToRender map[string]interface{},
	alreadyRenderedVariables map[string]interface{},
) (map[string]interface{}, error) {
	// Force to use ExitWithError for missing key, because by design this algorithm depends on boilerplate error-ing if
	// a variable can't be rendered due to a reference that hasn't been rendered yet. If OnMissingKey was invalid or
	// zero, then boilerplate will automatically render all references to `"not-valid"` or `""` in the first pass.
	//
	// We can do this because this option should only apply to the leaf variables (variables with no references), and
	// the leaf variables are handled by the time it gets to this function in the `alreadyRenderedVariables` map that is
	// passed in.
	//
	// NOTE: here, I am copying by value, not by reference by deferencing the pointer when assigning to optsForRender.
	// This ensures that opts (whatever caller passed in) doesn't change in this routine.
	optsForRender := *opts
	optsForRender.OnMissingKey = options.ExitWithError

	unrenderedVariables := []string{}
	for variableName := range variablesToRender {
		unrenderedVariables = append(unrenderedVariables, variableName)
	}

	var renderErr error
	renderedVariables := alreadyRenderedVariables
	rendered := true
	for iterations := 0; len(unrenderedVariables) > 0 && rendered; iterations++ {
		if iterations > MaxRenderAttempts {
			// Reached maximum supported iterations, which is most likely an infinite loop bug so cut the iteration
			// short an return an error.
			return nil, errors.WithStackTrace(MaxRenderAttemptsErr{})
		}

		attemptRenderOutput, err := attemptRenderVariables(&optsForRender, unrenderedVariables, renderedVariables, variablesToRender)
		unrenderedVariables = attemptRenderOutput.unrenderedVariables
		renderedVariables = attemptRenderOutput.renderedVariables
		rendered = attemptRenderOutput.variablesWereRendered
		renderErr = err
	}
	if len(unrenderedVariables) > 0 {
		return nil, renderErr
	}

	return renderedVariables, nil
}

// attemptRenderVariables is a helper function that drives the multiple trial algorithm. This represents a single trial
// of evaluating all the unrendered variables. This function goes through each unrendered variable and attempts to
// render them using the currently rendered variables. This will return:
// - all the variables that are still unrendered after this attempt
// - the updated map of rendered variables
// - a boolean indicating whether any new variables were rendered
func attemptRenderVariables(
	opts *options.BoilerplateOptions,
	unrenderedVariables []string,
	renderedVariables map[string]interface{},
	variables map[string]interface{},
) (attemptRenderVariablesOutput, error) {
	newUnrenderedVariables := []string{}
	wasRendered := false

	var allRenderErr error
	for _, variableName := range unrenderedVariables {
		rendered, err := attemptRenderVariable(opts, variables[variableName], renderedVariables)
		if err != nil {
			newUnrenderedVariables = append(newUnrenderedVariables, variableName)
			allRenderErr = multierror.Append(allRenderErr, err)
		} else {
			renderedVariables[variableName] = rendered
			wasRendered = true
		}
	}
	out := attemptRenderVariablesOutput{
		unrenderedVariables:   newUnrenderedVariables,
		renderedVariables:     renderedVariables,
		variablesWereRendered: wasRendered,
	}
	return out, allRenderErr
}

// attemptRenderVariable renders a single variable, using the provided renderedVariables to resolve any variable
// references.
// NOTE: This function is not responsible for converting the output type to the expected type configured on the
// boilerplate config, and will always use string as the primitive output.
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

// Return types

type attemptRenderVariablesOutput struct {
	unrenderedVariables   []string
	renderedVariables     map[string]interface{}
	variablesWereRendered bool
}

// Custom error types

type MaxRenderAttemptsErr struct{}

func (err MaxRenderAttemptsErr) Error() string {
	return fmt.Sprintf(`Reached maximum supported iterations for rendering variables. This can happen if you have:
- cyclic variable references
- deeper than supported variable references (max depth: %d)
`, MaxRenderAttempts)
}
