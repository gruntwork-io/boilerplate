package generator

import (
	"text/template"
	"bytes"
	"github.com/gruntwork-io/boilerplate/errors"
)

func RenderTemplate(templateName string, templateContents string, variables map[string]string) (string, error) {
	// TODO: add file and snippet helpers
	tmpl, err := template.New(templateName).Parse(templateContents)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, variables); err != nil {
		return "", errors.WithStackTrace(err)
	}

	return output.String(), nil
}
