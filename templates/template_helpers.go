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
	"math"
	"strconv"
	"unicode"
	"github.com/gruntwork-io/boilerplate/util"
	"sort"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/variables"
)

var SNIPPET_MARKER_REGEX = regexp.MustCompile("boilerplate-snippet:\\s*(.+?)(?:\\s|$)")

var WHITESPACE_REGEX = regexp.MustCompile("[[:space:]]+")

var PUNCTUATION_OR_WHITESPACE_REGEX = regexp.MustCompile("([[:space:]]|[[:punct:]])+")

// This regex can be used to split CamelCase strings into "words". That is, given a string like FooBarBaz, you can use
// this regex to split it into an array ["Foo", "Bar", "Baz"]. It also handles lower camel case, which is the same as
// camel case, except it starts with a lower case word, such as fooBarBaz.
//
// To capture lowercase camel case, we just look for words that consist of lower case letters and digits at the start
// of the string. To capture all other camel case, we look for "words" that start with one or more consecutive upper
// case letters followed by one or more lower case letters or digits.
var CAMEL_CASE_REGEX = regexp.MustCompile(
	"(^([[:lower:]]|[[:digit:]])+)|" +             // Handle lower camel case
	"([[:upper:]]*([[:lower:]]|[[:digit:]]|$)*)")  // Handle normal camel case

// All boilerplate template helpers implement this signature. They get the path of the template they are rendering as
// the first arg and then any arguments the user passed when calling the helper.
type TemplateHelper func(templatePath string, args ... string) (string, error)

// Create a map of custom template helpers exposed by boilerplate
func CreateTemplateHelpers(templatePath string, options *config.BoilerplateOptions, rootConfig *config.BoilerplateConfig) template.FuncMap {
	return map[string]interface{}{
		"snippet": wrapWithTemplatePath(templatePath, snippet),
		"downcase": strings.ToLower,
		"upcase": strings.ToUpper,
		"capitalize": strings.Title,
		"replace": func(old string, new string, str string) string { return strings.Replace(str, old, new, 1) },
		"replaceAll": func(old string, new string, str string) string { return strings.Replace(str, old, new, -1) },
		"trim": strings.TrimSpace,
		"round": wrapFloatToIntFunction(round),
		"ceil": wrapFloatToFloatFunction(math.Ceil),
		"floor": wrapFloatToFloatFunction(math.Floor),
		"dasherize": dasherize,
		"snakeCase": snakeCase,
		"camelCase": camelCase,
		"camelCaseLower": camelCaseLower,
		"plus": wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 + arg2 }),
		"minus": wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 - arg2 }),
		"times": wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 * arg2 }),
		"divide": wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 / arg2 }),
		"mod": wrapIntIntToIntFunction(func(arg1 int, arg2 int) int { return arg1 % arg2 }),
		"slice": slice,
		"keys": keys,
		"shell": wrapWithTemplatePath(templatePath, shell),
		"templateFolder": func() string { return options.TemplateFolder },
		"outputFolder": func() string { return options.OutputFolder },
		"dependencyOutputFolder": dependencyOutputFolder(rootConfig.Dependencies),
		//"dependencyOutputFolder": func() string { return rootConfig.Dependencies[0].OutputFolder },
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
	} else if util.IsDir(templatePath) {
		return filepath.Join(templatePath, filePath)
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

// Wrap a function that uses float64 as input and output so it can take any number as input and return a float64 as
// output
func wrapFloatToFloatFunction(f func(float64) float64) func(interface{}) (float64, error) {
	return func(value interface{}) (float64, error) {
		valueAsFloat, err := toFloat64(value)
		if err != nil {
			return 0, errors.WithStackTrace(err)
		}
		return f(valueAsFloat), nil
	}
}

// Wrap a function that uses float64 as input and int as output so it can take any number as input and return an int as
// output
func wrapFloatToIntFunction(f func(float64) int) func(interface{}) (int, error) {
	return func(value interface{}) (int, error) {
		valueAsFloat, err := toFloat64(value)
		if err != nil {
			return 0, errors.WithStackTrace(err)
		}
		return f(valueAsFloat), nil
	}
}

// Wrap a function that takes two ints as input and returns an int as output so it can take any kind of number as input
// and return an int as output
func wrapIntIntToIntFunction(f func(int, int) int) func(interface{}, interface{}) (int, error) {
	return func(arg1 interface{}, arg2 interface{}) (int, error) {
		arg1AsInt, err := toInt(arg1)
		if err != nil {
			return 0, errors.WithStackTrace(err)
		}
		arg2AsInt, err := toInt(arg2)
		if err != nil {
			return 0, errors.WithStackTrace(err)
		}
		return f(arg1AsInt, arg2AsInt), nil

	}
}

// Wrap a function that takes two float64's as input, performs arithmetic on them, and returns another float64 as a
// function that can take two values of any number kind as input and return a float64 as output
func wrapFloatFloatToFloatFunction(f func(arg1 float64, arg2 float64) float64) func(interface{}, interface{}) (float64, error) {
	return func(arg1 interface{}, arg2 interface{}) (float64, error) {
		arg1AsFloat, err := toFloat64(arg1)
		if err != nil {
			return 0, errors.WithStackTrace(err)
		}
		arg2AsFloat, err := toFloat64(arg2)
		if err != nil {
			return 0, errors.WithStackTrace(err)
		}
		return f(arg1AsFloat, arg2AsFloat), nil

	}
}

// Convert the given value to a float64. Does a proper conversion if the underlying type is a number. For all other
// types, we first convert to a string, and then try to parse the result as a float64.
func toFloat64(value interface{}) (float64, error) {
	// Because Go is a shitty language, we have to call out each of the numeric types separately, even though the
	// behavior for almost all of them is identical. If we tried to do a case statement with multiple clauses
	// (separated by comma), then the variable v would be of type interface{} and we could not use float64(..) to
	// convert it.
	switch v := value.(type) {
	case int: return float64(v), nil
	case int8: return float64(v), nil
	case int16: return float64(v), nil
	case int32: return float64(v), nil
	case int64: return float64(v), nil
	case uint: return float64(v), nil
	case uint8: return float64(v), nil
	case uint16: return float64(v), nil
	case uint32: return float64(v), nil
	case uint64: return float64(v), nil
	case float32: return float64(v), nil
	case float64: return v, nil
	default: return strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
	}
}

// Convert the given value to an int. Does a proper conversion if the underlying type is a number. For all other
// types, we first convert to a string, and then try to parse the result as a int.
func toInt(value interface{}) (int, error) {
	// Because Go is a shitty language, we have to call out each of the numeric types separately, even though the
	// behavior for almost all of them is identical. If we tried to do a case statement with multiple clauses
	// (separated by comma), then the variable v would be of type interface{} and we could not use int(..) to
	// convert it.
	switch v := value.(type) {
	case int: return v, nil
	case int8: return int(v), nil
	case int16: return int(v), nil
	case int32: return int(v), nil
	case int64: return int(v), nil
	case uint: return int(v), nil
	case uint8: return int(v), nil
	case uint16: return int(v), nil
	case uint32: return int(v), nil
	case uint64: return int(v), nil
	case float32: return int(v), nil
	case float64: return int(v), nil
	default: return strconv.Atoi(fmt.Sprintf("%v", v))
	}
}

// Go's math package does not include a round function. This is because Go is a shitty language. Many people complained
// in this issue: https://github.com/golang/go/issues/4594. However, it was closed as "wontfix". Note the half dozen
// attempts to implement this function, most of which are wrong. This seems to be the right solution:
func round(f float64) int {
	if math.Abs(f) < 0.5 {
		return 0
	}
	return int(f + math.Copysign(0.5, f))
}

// Convert a string to an all lowercase, dash-delimited string, dropping all other punctuation and whitespace. E.g.
// "foo BAR baz!" becomes "foo-bar-baz".
func dasherize(str string) string {
	return toDelimitedString(str, "-")
}

// Convert a string to an all lower case, underscore-delimited string, dropping all other punctuation and whitespace.
// E.g. "foo BAR baz!" becomes "foo_bar_baz".
func snakeCase(str string) string {
	return toDelimitedString(str, "_")
}

// Convert a string to camel case, dropping all punctuation and whitespace. E.g. "foo BAR baz!" becomes "FooBarBaz".
func camelCase(str string) string {
	// First, we strip any leading or trailing white space, underscores, or dashes
	trimmed := trimWhiteSpaceAndPunctuation(str)

	// Next, any time we find whitespace or punctuation repeated consecutively more than once, we replace
	// them with a single space.
	collapsed := collapseWhiteSpaceAndPunctuationToDelimiter(trimmed, " ")

	// Now we split on whitespace to find all the words in the string
	words := WHITESPACE_REGEX.Split(collapsed, -1)

	// Capitalize each word
	capitalized := []string{}
	for _, word := range words {
		capitalized = append(capitalized, strings.Title(word))
	}

	// Join everything back together into a string
	return strings.Join(capitalized, "")
}

// Convert a string to camel case where the first letter is lower case, dropping all punctuation and whitespace. E.g.
// "foo BAR baz!" becomes "fooBarBaz".
func camelCaseLower(str string) string {
	return lowerFirst(camelCase(str))
}

// Returns a copy of str with the first character converted to lower case. E.g. "FOO" becomes "fOO".
func lowerFirst(str string) string {
	if len(str) == 0 {
		return str
	}

	chars := []rune(str)
	chars[0] = unicode.ToLower(chars[0])
	return string(chars)
}

// This function converts a string to an all lower case string delimited with the given delimiter, dropping all other
// punctuation and whitespace. For example, "foo BAR baz" and the delimiter "-" would become "foo-bar-baz".
// TODO: handle all punctuation
func toDelimitedString(str string, delimiter string) string {
	// Although this function doesn't look terribly complicated, the reason why it's written this way,
	// unfortunately, isn't terribly obvious, so I'm going to use copious comments to try to build some intuition.

	// First, we strip any leading or trailing whitespace or punctuation
	trimmed := trimWhiteSpaceAndPunctuation(str)

	// Next, any time we find whitespace or punctuation repeated consecutively more than once, we replace
	// them with a single space. This space is really just being used as a placeholder between "words." In the next
	// step, these placeholders will be replaced with the delimiter.
	collapsed := collapseWhiteSpaceAndPunctuationToDelimiter(trimmed, " ")

	// The final step is to split the string into individual camel case "words" (see the comments on
	// CAMEL_CASE_REGEX for details on how it works) so that something like "FooBarBaz" becomes the array
	// ["Foo", "Bar", "Baz"]. We then combine all these words with the delimiter in between them ("Foo-Bar-Baz")
	// and convert the entire string to lower case ("foo-bar-baz").
	return strings.ToLower(strings.Join(CAMEL_CASE_REGEX.FindAllString(collapsed, -1), delimiter))
}

// Returns str with all leading and trailing whitespace and punctuation removed. E.g. "   foo!!!" becomes "foo".
func trimWhiteSpaceAndPunctuation(str string) string {
	return strings.TrimFunc(str, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
}

// Returns str with all consecutive whitespace and punctuation in it collapsed to the given delimiter. E.g.
// "foo.....bar_____baz" with a delimiter "-" becomes "foo-bar-baz".
func collapseWhiteSpaceAndPunctuationToDelimiter(str string, delimiter string) string {
	return PUNCTUATION_OR_WHITESPACE_REGEX.ReplaceAllString(str, delimiter)
}

// Generate a slice from start (inclusive) to end (exclusive), incrementing by increment. For example, slice(0, 5, 1)
// returns [0, 1, 2, 3, 4].
func slice(start interface{}, end interface{}, increment interface{}) ([]int, error) {
	out := []int{}

	startAsInt, err := toInt(start)
	if err != nil {
		return out, errors.WithStackTrace(err)
	}

	endAsInt, err := toInt(end)
	if err != nil {
		return out, errors.WithStackTrace(err)
	}

	incrementAsInt, err := toInt(increment)
	if err != nil {
		return out, errors.WithStackTrace(err)
	}

	for i := startAsInt; i < endAsInt; i += incrementAsInt {
		out = append(out, i)
	}

	return out, nil
}

// Return the keys in the given map. This method always returns the keys in sorted order to provide a stable iteration
// order.
func keys(m map[string]string) []string {
	out := []string{}

	for key, _ := range m {
		out = append(out, key)
	}

	sort.Strings(out)

	return out
}

// Run the given shell command specified in args in the working dir specified by templatePath and return stdout as a
// string.
func shell(templatePath string, args ... string) (string, error) {
	if len(args) == 0 {
		return "", errors.WithStackTrace(NoArgsPassedToShellHelper)
	}

	return util.RunShellCommandAndGetOutput(filepath.Dir(templatePath), args[0], args[1:]...)
}

// Look up a Dependency by name (most likely as declared in a boilerplate.yml), and return the outputFolder value, but
// with the given stringToDelete removed from the value.
// This is useful to extract a relative path from a particular outputFolder dependency.
func dependencyOutputFolder(dependencies []variables.Dependency) func(string, string) string {
	return func(dependencyName string, stringToDelete string) string {
		var outputFolder string

		for _, dependency := range dependencies {
			if dependency.Name == dependencyName {
				outputFolder = dependency.OutputFolder
				break;
			}
		}

		outputFolder = strings.Replace(outputFolder, stringToDelete, "", 1)

		return outputFolder
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

var NoArgsPassedToShellHelper = fmt.Errorf("The shell helper requires at least one argument")