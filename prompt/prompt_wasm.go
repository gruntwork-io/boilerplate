//go:build js && wasm

package prompt

import "fmt"

// PromptUserForInput is a stub for WASM builds where interactive prompts are not supported.
func PromptUserForInput(p string) (string, error) {
	return "", fmt.Errorf("interactive prompts are not supported in WASM")
}

// PromptUserForYesNo is a stub for WASM builds where interactive prompts are not supported.
func PromptUserForYesNo(p string) (bool, error) {
	return false, fmt.Errorf("interactive prompts are not supported in WASM")
}

// PromptUserForYesNoAll is a stub for WASM builds where interactive prompts are not supported.
func PromptUserForYesNoAll(p string) (UserResponse, error) {
	return UserResponseNo, fmt.Errorf("interactive prompts are not supported in WASM")
}
