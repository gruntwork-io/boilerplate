package templates

import (
	"io/ioutil"
	"github.com/gruntwork-io/boilerplate/errors"
	"fmt"
	"bufio"
	"os"
	"regexp"
	"text/template"
	"path/filepath"
	"strings"
	"path"
)

var SNIPPET_MARKER_REGEX = regexp.MustCompile("boilerplate-snippet:\\s*(.+?)(?:\\s|$)")

// All boilerplate template helpers implement this signature. They get the path of the template they are rendering as
// the first arg and then any arguments the user passed when calling the helper.
type TemplateHelper func(templatePath string, args ... string) (string, error)

// Create a map of custom template helpers exposed by boilerplate
func CreateTemplateHelpers(templatePath string) template.FuncMap {
	return map[string]interface{}{
		"snippet": wrapWithTemplatePath(templatePath, snippet),
	}
}

// When writing a template, it's natural to use a relative path, such as:
//
// {{snippet "../../foo/bar"}}
//
// However, this only works if boilerplate is called from the same folder as the template itself. To work around this
// issue, this function can be used to wrap boilerplate template helpers to make the path of the template itself
// available as the first argument to the helper. The helper can use that path to relativize other paths, if necessary.
func wrapWithTemplatePath(templatePath string, helper TemplateHelper) func(...string) (string, error) {
	return func(args ... string) (string, error) {
		return helper(templatePath, args...)
	}
}

// This helper expects the following args:
//
// snippet <TEMPLATE_PATH> <PATH> [SNIPPET_NAME]
//
// It returns the contents of PATH, relative to TEMPLATE_PATH, as a string. If SNIPPET_NAME is specified, only the
// contents of that snippet with that name will be returned. A snippet is any text in the file surrounded by a line on
// each side of the format "boilerplate-snippet: NAME" (typically using the comment syntax for the language).
func snippet(templatePath string, args ... string) (string, error) {
	switch len(args) {
	case 1: return readFile(templatePath, args[0])
	case 2: return readSnippetFromFile(templatePath, args[0], args[1])
	default: return "", errors.WithStackTrace(InvalidSnippetArguments(args))
	}
}

// Returns the given filePath relative to the given templatePath. If filePath is already an absolute path, returns it
// unchanged.
//
// Example:
//
// pathRelativeToTemplate("/foo/bar/template-file.txt, "../src/code.java")
//   Returns: "/foo/src/code.java"
func pathRelativeToTemplate(templatePath string, filePath string) string {
	if path.IsAbs(filePath) {
		return filePath
	} else {
		templateDir := filepath.Dir(templatePath)
		return filepath.Join(templateDir, filePath)
	}
}

// Returns the contents of the file at path, relative to templatePath, as a string
func readFile(templatePath, path string) (string, error) {
	relativePath := pathRelativeToTemplate(templatePath, path)
	bytes, err := ioutil.ReadFile(relativePath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	return string(bytes), nil
}

// Returns the contents of snippet snippetName from the file at path, relative to templatePath.
func readSnippetFromFile(templatePath string, path string, snippetName string) (string, error) {
	relativePath := pathRelativeToTemplate(templatePath, path)
	file, err := os.Open(relativePath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	return readSnippetFromScanner(scanner, snippetName)
}

// Returns the content of snippet snippetName from the given scanner
func readSnippetFromScanner(scanner *bufio.Scanner, snippetName string) (string, error) {
	scanner.Split(bufio.ScanLines)

	snippetLines := []string{}
	inSnippet := false

	for scanner.Scan() {
		line := scanner.Text()
		foundSnippetName, isSnippet := extractSnippetName(line)
		if isSnippet && foundSnippetName == snippetName {
			if inSnippet {
				return strings.Join(snippetLines, "\n"), nil
			} else {
				inSnippet = true
			}
		} else if inSnippet {
			snippetLines = append(snippetLines, line)
		}
	}

	if inSnippet {
		return "", errors.WithStackTrace(SnippetNotTerminated(snippetName))
	} else {
		return "", errors.WithStackTrace(SnippetNotFound(snippetName))
	}
}

// Extract the snippet name from the line of text. A snippet is of the form "boilerplate-snippet: NAME". If no snippet
// name is found, return false for the second argument.
func extractSnippetName(line string) (string, bool) {
	match := SNIPPET_MARKER_REGEX.FindStringSubmatch(line)
	if len(match) == 2 {
		snippetName := strings.TrimSpace(match[1])
		return snippetName, snippetName != ""
	} else {
		return "", false
	}
}

// Custom errors

type SnippetNotFound string
func (snippetName SnippetNotFound) Error() string {
	return fmt.Sprintf("Could not find a snippet named %s", string(snippetName))
}

type SnippetNotTerminated string
func (snippetName SnippetNotTerminated) Error() string {
	return fmt.Sprintf("Snippet %s has an opening boilerplate-snippet marker, but not a closing one", string(snippetName))
}

type InvalidSnippetArguments []string
func (args InvalidSnippetArguments) Error() string {
	return fmt.Sprintf("The snippet helper expects the following args: snippet <TEMPLATE_PATH> <PATH> [SNIPPET_NAME]. Instead, got args: %s", []string(args))
}