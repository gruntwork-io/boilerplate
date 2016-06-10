package templates

import (
	"text/template"
	"bytes"
	"github.com/gruntwork-io/boilerplate/errors"
	"path"
	"strings"
	"os"
	"path/filepath"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/util"
	"io/ioutil"
)

func ProcessTemplateFolder(templateFolder string, outputFolder string, variables map[string]string) error {
	util.Logger.Printf("Processing templates in %s and outputting generated files to %s", templateFolder, outputFolder)

	if err := os.MkdirAll(outputFolder, 0777); err != nil {
		return errors.WithStackTrace(err)
	}

	return filepath.Walk(templateFolder, func(path string, info os.FileInfo, err error) error {
		if shouldSkipPath(path, templateFolder) {
			util.Logger.Printf("Skipping %s", path)
			return nil
		} else {
			return processPath(path, templateFolder, outputFolder, variables)
		}
	})
}

func processPath(path string, templateFolder string, outputFolder string, variables map[string]string) error {
	isText, err := util.IsTextFile(path)
	if err != nil {
		return err
	}

	if isText {
		return processTemplate(path, templateFolder, outputFolder, variables)
	} else {
		return copyFile(path, templateFolder, outputFolder, variables)
	}
}

func outPath(file string, templateFolder string, outputFolder string) string {
	// TODO process template syntax in paths
	relativePath := strings.TrimPrefix(file, templateFolder)
	return path.Join(outputFolder, relativePath)
}

func copyFile(file string, templateFolder string, outputFolder string, variables map[string]string) error {
	destination := outPath(file, templateFolder, outputFolder)
	util.Logger.Printf("Copying %s to %s", file, destination)
	return util.CopyFile(file, destination)
}

func processTemplate(templatePath string, templateFolder string, outputFolder string, variables map[string]string) error {
	destination := outPath(templatePath, templateFolder, outputFolder)
	util.Logger.Printf("Processing template %s and writing to %s", templatePath, destination)

	bytes, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	out, err := renderTemplate(templatePath, string(bytes), variables)
	if err != nil {
		return err
	}

	return util.WriteFileWithSamePermissions(templatePath, destination, []byte(out))
}

func shouldSkipPath(path string, templateFolder string) bool {
	return path == templateFolder || path == config.BoilerPlateConfigPath(templateFolder)
}

func renderTemplate(templateName string, templateContents string, variables map[string]string) (string, error) {
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
