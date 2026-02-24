//go:build js && wasm

// Package validation provides input validation rules for boilerplate variables.
package validation

// CustomValidationRule is a WASM-compatible stub. The Validator field uses `any`
// instead of `validation.Rule` to avoid pulling in ozzo-validation (and its
// transitive crypto dependencies) into the WASM binary.
type CustomValidationRule struct {
	Validator any
	Message   string
	Args      []any // Original arguments for parameterized rules (e.g., regex pattern, length bounds).
}

// CustomValidationRuleCollection is a slice of CustomValidationRule.
type CustomValidationRuleCollection []CustomValidationRule

// DescriptionText returns the human-readable message for the validation rule.
func (c CustomValidationRule) DescriptionText() string {
	return c.Message
}

// UnmarshalValidationsField is a stub for WASM builds. Validation is never
// performed in the WASM environment. This is called from
// UnmarshalVariableFromBoilerplateConfigYaml in variables.go, which compiles
// in WASM but is never executed at runtime.
func UnmarshalValidationsField(fields map[string]any) ([]CustomValidationRule, error) {
	return nil, nil
}
