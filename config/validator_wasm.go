//go:build js && wasm

package config

// runValidator is a no-op in WASM builds. See validator_cli.go for the
// non-WASM implementation. WASM never runs validation; the validation package
// stub records rules but does not exercise them.
func runValidator(_ any, _ any) error {
	return nil
}
