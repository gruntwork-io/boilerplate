//go:build !(js && wasm)

// Package validation provides input validation rules for boilerplate variables.
package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
)

var errInvalidRegexPattern = errors.New("pattern must be a quoted string (e.g. regex(\"pattern\") or regex(`pattern`))")

// CustomValidationRule represents a single validation rule with its ozzo-validation
// validator and a human-readable description message.
type CustomValidationRule struct {
	Validator validation.Rule
	Message   string
}

// CustomValidationRuleCollection is a slice of CustomValidationRule.
type CustomValidationRuleCollection []CustomValidationRule

// GetValidators returns just the ozzo-validation Rule values from the collection.
func (c CustomValidationRuleCollection) GetValidators() []validation.Rule {
	validatorsToReturn := make([]validation.Rule, 0, len(c))
	for _, rule := range c {
		validatorsToReturn = append(validatorsToReturn, rule.Validator)
	}

	return validatorsToReturn
}

// DescriptionText returns the human-readable message for the validation rule.
func (c CustomValidationRule) DescriptionText() string {
	return c.Message
}

// convertSingleValidationRule converts a single validation rule string into a CustomValidationRule.
// Rule names are case-sensitive and must match exactly (e.g. "required", not "Required").
func convertSingleValidationRule(rule string) (CustomValidationRule, error) {
	rule = normalizeRuleString(rule)

	switch {
	case rule == "required":
		return CustomValidationRule{
			Validator: validation.Required,
			Message:   "Must not be empty",
		}, nil
	case rule == "url":
		return CustomValidationRule{
			Validator: is.URL,
			Message:   "Must be a valid URL",
		}, nil
	case rule == "email":
		return CustomValidationRule{
			Validator: is.Email,
			Message:   "Must be a valid email address",
		}, nil
	case rule == "alpha":
		return CustomValidationRule{
			Validator: is.Alpha,
			Message:   "Must contain English letters only",
		}, nil
	case rule == "digit":
		return CustomValidationRule{
			Validator: is.Digit,
			Message:   "Must contain digits only",
		}, nil
	case rule == "alphanumeric":
		return CustomValidationRule{
			Validator: is.Alphanumeric,
			Message:   "Can contain English letters and digits only",
		}, nil
	case rule == "countrycode2":
		return CustomValidationRule{
			Validator: is.CountryCode2,
			Message:   "Must be a valid ISO3166 Alpha 2 Country code",
		}, nil
	case rule == "semver":
		return CustomValidationRule{
			Validator: is.Semver,
			Message:   "Must be a valid semantic version",
		}, nil
	case strings.HasPrefix(rule, "length(") && strings.HasSuffix(rule, ")"):
		// lengthArgCount is the expected number of arguments for the length() validation rule.
		const lengthArgCount = 2

		inner := strings.TrimSuffix(strings.TrimPrefix(rule, "length("), ")")
		parts := strings.SplitN(inner, ",", lengthArgCount)

		if len(parts) != lengthArgCount {
			return CustomValidationRule{}, fmt.Errorf("invalid length validation %q: expected length(min, max)", rule)
		}

		min, minErr := strconv.Atoi(strings.TrimSpace(parts[0]))
		if minErr != nil {
			return CustomValidationRule{}, fmt.Errorf("invalid min in length validation %q: %w", rule, minErr)
		}

		max, maxErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if maxErr != nil {
			return CustomValidationRule{}, fmt.Errorf("invalid max in length validation %q: %w", rule, maxErr)
		}

		if min > max {
			return CustomValidationRule{}, fmt.Errorf("invalid length validation %q: min must be less than max", rule)
		}

		return CustomValidationRule{
			Validator: validation.Length(min, max),
			Message:   fmt.Sprintf("Must be between %d and %d characters long", min, max),
		}, nil
	case strings.HasPrefix(rule, "regex(") && strings.HasSuffix(rule, ")"):
		quoted := rule[len("regex(") : len(rule)-1]

		pattern, err := unquoteRegexPattern(quoted)
		if err != nil {
			return CustomValidationRule{}, fmt.Errorf(
				"invalid regex validation %q: %w", rule, err)
		}

		compiledRegex, err := regexp.Compile(pattern)
		if err != nil {
			return CustomValidationRule{}, fmt.Errorf("invalid regex pattern in validation %q: %w", pattern, err)
		}

		return CustomValidationRule{
			Validator: validation.Match(compiledRegex),
			Message:   "Must match pattern: " + pattern,
		}, nil
	default:
		return CustomValidationRule{}, fmt.Errorf("unrecognized validation rule %q", rule)
	}
}

// unquoteRegexPattern extracts a regex pattern from a quoted string. It accepts
// double-quoted ("pattern") and backtick-quoted (`pattern`) strings. Unlike
// strconv.Unquote, double-quoted strings are treated as nearly raw: only \"
// and \\ are interpreted as escape sequences, so regex metacharacters like \d,
// \w, and \s pass through without requiring double-escaping.
func unquoteRegexPattern(quoted string) (string, error) {
	const minQuotedLen = 2

	if len(quoted) < minQuotedLen {
		return "", errInvalidRegexPattern
	}

	opener, closer := quoted[0], quoted[len(quoted)-1]

	switch {
	case opener == '`' && closer == '`':
		return quoted[1 : len(quoted)-1], nil

	case opener == '"' && closer == '"':
		raw := quoted[1 : len(quoted)-1]

		var buf strings.Builder

		buf.Grow(len(raw))

		for i := 0; i < len(raw); i++ {
			if raw[i] == '\\' && i+1 < len(raw) {
				next := raw[i+1]
				if next == '"' || next == '\\' {
					buf.WriteByte(next)

					i++

					continue
				}
			}

			if raw[i] == '"' {
				return "", errors.New(
					"unescaped \" in regex pattern; use \\\" to include a literal quote, or use backtick quoting: regex(`pattern`)")
			}

			buf.WriteByte(raw[i])
		}

		return buf.String(), nil

	default:
		return "", errInvalidRegexPattern
	}
}

// normalizeRuleString trims surrounding whitespace from a validation rule string.
// Rule names and arguments are case-sensitive and must be written exactly as documented.
func normalizeRuleString(rule string) string {
	return strings.TrimSpace(rule)
}

// UnmarshalValidationsField looks up the validations specified in the map and converts them to
// CustomValidationRules that provide real-time feedback on the validity of user entries.
// The recommended YAML format is a list of rules:
//
//	validations:
//	  - required
//	  - regex("^[a-z ]+$")
//
// A scalar string (e.g. "validations: required") is ignored and a warning is emitted.
func UnmarshalValidationsField(fields map[string]any) ([]CustomValidationRule, error) {
	validations := fields["validations"]
	if validations == nil {
		return nil, nil
	}

	switch v := validations.(type) {
	// We use []any (rather than []string) because Go's YAML libraries unmarshal
	// sequences into []any, not []string, when the target type is any/interface{}.
	case []any:
		// List of rules (e.g., ["required", `regex("^[a-z ]+$")`])
		// Process each element individually to preserve spaces and brackets in patterns
		allRules := make([]CustomValidationRule, 0, len(v))

		for i, item := range v {
			var ruleStr string

			switch val := item.(type) {
			case string:
				ruleStr = val
			case fmt.Stringer:
				ruleStr = val.String()
			default:
				return nil, fmt.Errorf("validation rule at index %d must be a string, got %T", i, item)
			}

			rule := normalizeRuleString(ruleStr)

			cvr, err := convertSingleValidationRule(rule)
			if err != nil {
				return nil, err
			}

			allRules = append(allRules, cvr)
		}

		return allRules, nil
	case string:
		return nil, fmt.Errorf("the 'validations' field must be a YAML list, not a string (%q). "+
			"Please use the list format instead:\n  validations:\n    - %q", v, v)
	default:
		return nil, fmt.Errorf("validations field must be a list of strings, got %T", validations)
	}
}

// ValidationsMissing is an error type for when validations are expected but not specified.
type ValidationsMissing string

func (err ValidationsMissing) Error() string {
	return string(err) + " does not specify any validations. You must specify at least one validation."
}
