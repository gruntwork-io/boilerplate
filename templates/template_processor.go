package templates

import (
	"text/template"
	"bytes"
	"github.com/gruntwork-io/boilerplate/errors"
	"os"
	"path/filepath"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/util"
	"io/ioutil"
	"path"
)

// Copy all the files and folders in templateFolder to outputFolder, passing text files through the Go template engine
// with the given set of variables as the data.
func ProcessTemplateFolder(options *config.BoilerplateOptions, variables map[string]string) error {
	util.Logger.Printf("Processing templates in %s and outputting generated files to %s", options.TemplateFolder, options.OutputFolder)

	if err := os.MkdirAll(options.OutputFolder, 0777); err != nil {
		return errors.WithStackTrace(err)
	}

	return filepath.Walk(options.TemplateFolder, func(path string, info os.FileInfo, err error) error {
		if shouldSkipPath(path, options) {
			util.Logger.Printf("Skipping %s", path)
			return nil
		} else if util.IsDir(path) {
			return createOutputDir(path, options)
		} else {
			return processFile(path, options, variables)
		}
	})
}

// Copy the given path, which is in the folder templateFolder, to the outputFolder, passing it through the Go template
// engine with the given set of variables as the data if it's a text file.
func processFile(path string, options *config.BoilerplateOptions, variables map[string]string) error {
	isText, err := util.IsTextFile(path)
	if err != nil {
		return err
	}

	if isText {
		return processTemplate(path, options, variables)
	} else {
		return copyFile(path, options, variables)
	}
}

// Create the given directory, which is in templateFolder, in the given outputFolder
func createOutputDir(dir string, options *config.BoilerplateOptions) error {
	destination, err := outPath(dir, options.TemplateFolder, options.OutputFolder)
	if err != nil {
		return err
	}

	util.Logger.Printf("Creating folder %s", destination)
	return os.MkdirAll(destination, 0777)
}

// Compute the path where the given file, which is in templateFolder, should be copied in outputFolder
func outPath(file string, templateFolder string, outputFolder string) (string, error) {
	// TODO process template syntax in paths
	templateFolderAbsPath, err := filepath.Abs(templateFolder)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	fileAbsPath, err := filepath.Abs(file)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	relPath, err := filepath.Rel(templateFolderAbsPath, fileAbsPath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return path.Join(outputFolder, relPath), nil
}

// Copy the given file, which is in options.TemplateFolder, to options.OutputFolder
func copyFile(file string, options *config.BoilerplateOptions, variables map[string]string) error {
	destination, err := outPath(file, options.TemplateFolder, options.OutputFolder)
	if err != nil {
		return err
	}

	util.Logger.Printf("Copying %s to %s", file, destination)
	return util.CopyFile(file, destination)
}

// Run the template at templatePath, which is in templateFolder, through the Go template engine with the given
// variables as data and write the result to outputFolder
func processTemplate(templatePath string, options *config.BoilerplateOptions, variables map[string]string) error {
	destination, err := outPath(templatePath, options.TemplateFolder, options.OutputFolder)
	if err != nil {
		return err
	}

	util.Logger.Printf("Processing template %s and writing to %s", templatePath, destination)
	bytes, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	out, err := renderTemplate(templatePath, string(bytes), variables, options.OnMissingKey)
	if err != nil {
		return err
	}

	return util.WriteFileWithSamePermissions(templatePath, destination, []byte(out))
}

// Return true if this is a path that should not be copied
func shouldSkipPath(path string, options *config.BoilerplateOptions) bool {
	return path == options.TemplateFolder || path == config.BoilerPlateConfigPath(options.TemplateFolder)
}

// Render the template at templatePath, with contents templateContents, using the Go template engine, passing in the
// given variables as data.
func renderTemplate(templatePath string, templateContents string, variables map[string]string, missingKeyAction config.MissingKeyAction) (string, error) {
	template := template.New(templatePath).Funcs(CreateTemplateHelpers(templatePath)).Option("missingkey=" + missingKeyAction.String())

	parsedTemplate, err := template.Parse(templateContents)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	var output bytes.Buffer
	if err := parsedTemplate.Execute(&output, variables); err != nil {
		return "", errors.WithStackTrace(err)
	}

	return output.String(), nil
}
