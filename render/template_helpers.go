package render

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/Masterminds/sprig/v3"

	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
	"gopkg.in/yaml.v2"
)

var SNIPPET_MARKER_REGEX = regexp.MustCompile("boilerplate-snippet:\\s*(.+?)(?:\\s|$)")

var WHITESPACE_REGEX = regexp.MustCompile("[[:space:]]+")

var PUNCTUATION_OR_WHITESPACE_REGEX = regexp.MustCompile("([[:space:]]|[[:punct:]])+")

var ENV_VAR_REGEX = regexp.MustCompile("^ENV:(.+?)=(.*)$")

const SHELL_DISABLED_PLACEHOLDER = "replace-me"

// This regex can be used to split CamelCase strings into "words". That is, given a string like FooBarBaz, you can use
// this regex to split it into an array ["Foo", "Bar", "Baz"]. It also handles lower camel case, which is the same as
// camel case, except it starts with a lower case word, such as fooBarBaz.
//
// To capture lowercase camel case, we just look for words that consist of lower case letters and digits at the start
// of the string. To capture all other camel case, we look for "words" that start with one or more consecutive upper
// case letters followed by one or more lower case letters or digits.
var CAMEL_CASE_REGEX = regexp.MustCompile(
	"(^([[:lower:]]|[[:digit:]])+)|" + // Handle lower camel case
		"([[:upper:]]*([[:lower:]]|[[:digit:]]|$)*)") // Handle normal camel case

// All boilerplate template helpers implement this signature. They get the path of the template they are rendering as
// the first arg, the Boilerplate Options as the second arg, and then any arguments the user passed when calling the
// helper.
type TemplateHelper func(templatePath string, opts *options.BoilerplateOptions, args ...string) (string, error)

// Create a map of custom template helpers exposed by boilerplate
func CreateTemplateHelpers(templatePath string, opts *options.BoilerplateOptions, tmpl *template.Template) template.FuncMap {
	sprigFuncs := sprig.FuncMap()
	// We rename a few sprig functions that overlap with boilerplate implementations. See DEPRECATED note on boilerplate
	// functions below for more details.
	sprigFuncs["listSlice"] = sprigFuncs["slice"]
	sprigFuncs["replaceAll"] = sprigFuncs["replace"]
	sprigFuncs["keysUnordered"] = sprigFuncs["keys"]
	sprigFuncs["readEnv"] = sprigFuncs["env"]
	sprigFuncs["roundFloat"] = sprigFuncs["round"]
	sprigFuncs["ceilFloat"] = sprigFuncs["ceil"]
	sprigFuncs["floorFloat"] = sprigFuncs["floor"]
	sprigFuncs["trimPrefixSprig"] = sprigFuncs["trimPrefix"]
	sprigFuncs["trimSuffixSprig"] = sprigFuncs["trimSuffix"]

	boilerplateFuncs := map[string]interface{}{
		"roundInt": wrapFloatToIntFunction(round),
		"ceilInt":  wrapFloatToFloatFunction(math.Ceil),
		"floorInt": wrapFloatToFloatFunction(math.Floor),

		"plus":   wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 + arg2 }),
		"minus":  wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 - arg2 }),
		"times":  wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 * arg2 }),
		"divide": wrapFloatFloatToFloatFunction(func(arg1 float64, arg2 float64) float64 { return arg1 / arg2 }),

		"dasherize":             dasherize,
		"camelCaseLower":        camelCaseLower,
		"replaceOne":            func(old string, new string, str string) string { return strings.Replace(str, old, new, 1) },
		"trimPrefixBoilerplate": trimPrefix,
		"trimSuffixBoilerplate": trimSuffix,
		"toYaml":                toYaml,

		"numRange":   slice,
		"keysSorted": keys,

		"snippet":    wrapWithTemplatePath(templatePath, opts, snippet),
		"include":    wrapIncludeWithTemplatePath(templatePath, opts),
		"shell":      wrapWithTemplatePath(templatePath, opts, shell),
		"pathExists": util.PathExists,

		"templateIsDefined": wrapIsDefinedWithTemplate(tmpl),

		"templateFolder":        func() (string, error) { return filepath.Abs(opts.TemplateFolder) },
		"outputFolder":          func() (string, error) { return filepath.Abs(opts.OutputFolder) },
		"relPath":               relPath,
		"boilerplateConfigDeps": boilerplateConfigDeps(opts),
		"boilerplateConfigVars": boilerplateConfigVars(opts),
		"envWithDefault":        env,

		// DEPRECATIONS

		// These functions are exactly the same as their sprig counterpart
		"downcase":   strings.ToLower, // lower
		"upcase":     strings.ToUpper, // upper
		"capitalize": strings.Title,   // title
		"snakeCase":  snakeCase,       // snakecase
		"camelCase":  camelCase,       // camelcase

		// In sprig, trimPrefix and trimSuffix take the arguments in different orders so that you can use pipelines. For backwards compatibility, we
		// have:
		// - trimPrefix : The original boilerplate version of trimPrefix.
		// - trimPrefixSprig : The sprig version of trimPrefix.
		// - trimPrefixBoilerplate : Another name for the boilerplate version of trimPrefix.
		// Users need to upgrade usage of `trimPrefix` with `trimPrefixBoilerplate`. The same with `trimSuffix`.
		"trimPrefix": trimPrefix,
		"trimSuffix": trimSuffix,

		// In sprig, round supports arbitrary decimal place rounding. E.g {{ round 123.5555 3 }} returns 123.556. For
		// backwards compatibility, we have:
		// - round : The original boilerplate round function that rounds to nearest int.
		// - roundFloat : The sprig version of round.
		// - roundInt : Another name for the boilerplate round function that doesn't overlap with sprig.
		// Users need to upgrade usage of `round` with `roundInt`.
		"round": wrapFloatToIntFunction(round),

		// In sprig, ceil and floor is functionally the same as the boilerplate versions, except they return floats as
		// opposed to ints. E.g {{ ceil 1.1 }} returns 2.0. For backwards compatibility, we have:
		// - ceil : The original boilerplate ceil function that truncates.
		// - ceilFloat : The sprig version of ceil.
		// - ceilInt : Another name for the boilerplate version of ceil that doesn't overlap with sprig.
		// Users need to update usage of `ceil` with `ceilInt`.
		// Note that the same function naming applies with floor.
		"ceil":  wrapFloatToFloatFunction(math.Ceil),
		"floor": wrapFloatToFloatFunction(math.Floor),

		// In sprig, env does not support default values. For backwards compatibility, we have:
		// - env : The original boilerplate env function that supports default value if env doesn't exist.
		// - readEnv : The sprig version of env.
		// - envWithDefault : Another name for the boilerplate env function that doesn't overlap with sprig.
		// Users need to upgrade usage of `env` with `envWithDefault`.
		"env": env,

		// In sprig, keys is the unordered keys version. To get the sorted version, you use
		// {{ keys $myDict | sortAlpha }}. For backwards compatibility, we have:
		// - keys : The original boilerplate keys function that returns the keys in sorted order.
		// - keysUnordered : The sprig version of keys.
		// - keysSorted : Another name for the boilerplate keys function that doesn't overlap with sprig.
		// Users need to upgrade usage of `keys` with `keysSorted`.
		"keys": keys,

		// In sprig, replace is replaceAll. For backwards compatibility, we have:
		// - replace : The original boilerplate replace function
		// - replaceAll : The sprig version of replace. Compatible with original boilerplate replaceAll.
		// - replaceOne : Another name for the boilerplate replace function that doesn't overlap with sprig.
		// Users need to upgrade usage of `replace` with `replaceOne`.
		"replace": func(old string, new string, str string) string { return strings.Replace(str, old, new, 1) },

		// In sprig, slice is the very useful anylist slice function, that takes a list and returns list[n:m].
		// For backwards compatibility, we have:
		// - slice : The original boilerplate slice function
		// - sliceList : The sprig version of slice
		// - numRange : Another name for the boilerplate slice function that doesn't overlap with sprig
		// Users need to upgrade usage of `slice` to `numRange`.
		"slice": slice,
	}

	funcs := map[string]interface{}{}
	for k, v := range sprigFuncs {
		funcs[k] = v
	}
	for k, v := range boilerplateFuncs {
		funcs[k] = v
	}
	return funcs
}

// When writing a template, it's natural to use a relative path, such as:
//
// {{snippet "../../foo/bar"}}
//
// However, this only works if boilerplate is called from the same folder as the template itself. To work around this
// issue, this function can be used to wrap boilerplate template helpers to make the path of the template itself
// available as the first argument and the BoilerplateOptions as the second argument. The helper can use that path to
// relativize other paths, if necessary.
func wrapWithTemplatePath(templatePath string, opts *options.BoilerplateOptions, helper TemplateHelper) func(...string) (string, error) {
	return func(args ...string) (string, error) {
		return helper(templatePath, opts, args...)
	}
}

// This works exactly like wrapWithTemplatePath, but it is adapted to the function args for the include helper function.
func wrapIncludeWithTemplatePath(templatePath string, opts *options.BoilerplateOptions) func(string, map[string]interface{}) (string, error) {
	return func(path string, varData map[string]interface{}) (string, error) {
		return include(templatePath, opts, path, varData)
	}
}

// wrapIsDefinedWithTemplate wraps templateIsDefined, passing in the current *Template to allow the
// function to introspect what templates have been defined
func wrapIsDefinedWithTemplate(tmpl *template.Template) func(string) bool {
	return func(name string) bool {
		return templateIsDefined(tmpl, name)
	}
}

// templateIsDefined determines whether a given template name has been defined or not, allowing
// boilerplate templates to conditionally include other templates.
func templateIsDefined(tmpl *template.Template, name string) bool {
	for _, templateName := range tmpl.Templates() {
		if templateName.Name() == name {
			return true
		}
	}
	return false
}

// This helper expects the following args:
//
// snippet <TEMPLATE_PATH> <PATH> [SNIPPET_NAME]
//
// It returns the contents of PATH, relative to TEMPLATE_PATH, as a string. If SNIPPET_NAME is specified, only the
// contents of that snippet with that name will be returned. A snippet is any text in the file surrounded by a line on
// each side of the format "boilerplate-snippet: NAME" (typically using the comment syntax for the language).
func snippet(templatePath string, opts *options.BoilerplateOptions, args ...string) (string, error) {
	switch len(args) {
	case 1:
		return readFile(templatePath, args[0])
	case 2:
		return readSnippetFromFile(templatePath, args[0], args[1])
	default:
		return "", errors.WithStackTrace(InvalidSnippetArguments(args))
	}
}

// This helper expects the following args:
//
// include <TEMPLATE_PATH> <PATH> <VARIABLES>
//
// This helper returns the contents of PATH, relative to TEMPLAT_PATH, but rendered through the boilerplate templating
// engine with the given variables.
func include(templatePath string, opts *options.BoilerplateOptions, path string, varData map[string]interface{}) (string, error) {
	templateContents, err := readFile(templatePath, path)
	if err != nil {
		return "", err
	}
	return RenderTemplateFromString(templatePath, templateContents, varData, opts)
}

// Returns the given filePath relative to the given templatePath. If filePath is already an absolute path, returns it
// unchanged.
//
// Example:
//
// pathRelativeToTemplate("/foo/bar/template-file.txt, "../src/code.java")
//
//	Returns: "/foo/src/code.java"
func PathRelativeToTemplate(templatePath string, filePath string) string {
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
	relativePath := PathRelativeToTemplate(templatePath, path)
	bytes, err := ioutil.ReadFile(relativePath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	return string(bytes), nil
}

// Returns the contents of snippet snippetName from the file at path, relative to templatePath.
func readSnippetFromFile(templatePath string, path string, snippetName string) (string, error) {
	relativePath := PathRelativeToTemplate(templatePath, path)
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
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
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
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return strconv.Atoi(fmt.Sprintf("%v", v))
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
func keys(value interface{}) ([]string, error) {
	valueType := reflect.ValueOf(value)
	if valueType.Kind() != reflect.Map {
		return nil, errors.WithStackTrace(InvalidTypeForMethodArgument{"keys", "Map", valueType.Kind().String()})
	}

	out := []string{}

	for _, key := range valueType.MapKeys() {
		out = append(out, fmt.Sprintf("%v", key.Interface()))
	}

	sort.Strings(out)

	return out, nil
}

// Run the given shell command specified in args in the working dir specified by templatePath and return stdout as a
// string.
func shell(templatePath string, opts *options.BoilerplateOptions, rawArgs ...string) (string, error) {
	if opts.DisableShell {
		opts.Logger.Info(fmt.Sprintf("Shell helpers are disabled. Will not execute shell command '%v'. Returning placeholder value '%s' instead.", rawArgs, SHELL_DISABLED_PLACEHOLDER))
		return SHELL_DISABLED_PLACEHOLDER, nil
	}

	if len(rawArgs) == 0 {
		return "", errors.WithStackTrace(NoArgsPassedToShellHelper)
	}

	args, envVars := separateArgsAndEnvVars(rawArgs)
	return util.RunShellCommandAndGetOutput(filepath.Dir(templatePath), envVars, args[0], args[1:]...)
}

// To pass env vars to the shell helper, we use the format ENV:KEY=VALUE. This method goes through the given list of
// arguments and splits it into two lists: the list of cmd-line args and the list of env vars.
func separateArgsAndEnvVars(rawArgs []string) ([]string, []string) {
	args := []string{}
	envVars := []string{}

	for _, rawArg := range rawArgs {
		matches := ENV_VAR_REGEX.FindStringSubmatch(rawArg)
		if len(matches) == 3 {
			key := matches[1]
			value := matches[2]
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
		} else {
			args = append(args, rawArg)
		}
	}

	return args, envVars
}

// Returns str without the provided leading prefix string. If str doesn't start with prefix, str is returned unchanged.
func trimPrefix(str, prefix string) string {
	return strings.TrimPrefix(str, prefix)
}

// Returns str without the provided trailing suffix string. If str doesn't end with suffix, str is returned unchanged.
func trimSuffix(str, suffix string) string {
	return strings.TrimPrefix(str, suffix)
}

func toYaml(obj interface{}) (string, error) {
	yamlObj, err := yaml.Marshal(&obj)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	return string(yamlObj), nil
}

// Returns the relative path between the output folders of a "base" path and a "target" path.
func relPath(basePath, targetPath string) (string, error) {
	relPath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return relPath, nil
}

// Returns the value of the environment variable with the given name. If that variable is not set, return fallbackValue.
func env(name string, fallbackValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallbackValue
	} else {
		return value
	}
}

// Find the value of the given property of the given Dependency.
func boilerplateConfigDeps(opts *options.BoilerplateOptions) func(string, string) (string, error) {
	return func(name string, property string) (string, error) {
		deps := opts.Vars["BoilerplateConfigDeps"].(map[string]variables.Dependency)
		dep := deps[name]

		if dep.Name == "" {
			return "", fmt.Errorf(`The dependency "%s" was not found.`, name)
		}

		r := reflect.ValueOf(dep)
		f := reflect.Indirect(r).FieldByName(property)
		return f.String(), nil
	}
}

// Find the value of the given property of the given Variable.
func boilerplateConfigVars(opts *options.BoilerplateOptions) func(string, string) (string, error) {
	return func(name string, property string) (string, error) {
		vars := opts.Vars["BoilerplateConfigVars"].(map[string]variables.Variable)
		myVar := vars[name]

		if myVar.Name() == "" {
			return "", fmt.Errorf(`The variable "%s" was not found.`, name)
		}

		r := reflect.ValueOf(myVar)
		f := reflect.Indirect(r).FieldByName(property)
		return f.String(), nil
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

type InvalidTypeForMethodArgument struct {
	MethodName   string
	ExpectedType string
	ActualType   string
}

func (err InvalidTypeForMethodArgument) Error() string {
	return fmt.Sprintf("Method %s expects type %s, but got %s", err.MethodName, err.ExpectedType, err.ActualType)
}
