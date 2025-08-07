package util

import (
	"bufio"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/inancgumus/screen"

	"github.com/gruntwork-io/boilerplate/errors"
)

var brightGreen = color.New(color.FgHiGreen, color.Bold)

// UserResponse represents the user's response to a yes/no/all prompt
type UserResponse string

const (
	UserResponseYes UserResponse = "yes"
	UserResponseNo  UserResponse = "no"
	UserResponseAll UserResponse = "all"
)

// PromptUserForInput prompts the user for text in the CLI. Returns the text entered by the user.
func PromptUserForInput(prompt string) (string, error) {
	if _, err := brightGreen.Print(prompt + ": "); err != nil {
		return "", errors.WithStackTrace(err)
	}

	reader := bufio.NewReader(os.Stdin)

	text, err := reader.ReadString('\n')
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return strings.TrimSpace(text), nil
}

// PromptUserForYesNo prompts the user for a yes/no response and returns true if they entered yes.
func PromptUserForYesNo(prompt string) (bool, error) {
	resp, err := PromptUserForInput(prompt + " (y/n) ")

	if err != nil {
		return false, errors.WithStackTrace(err)
	}

	switch strings.ToLower(resp) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// PromptUserForYesNoAll prompts the user for a y/a/n response and returns the response type.
// Returns: UserResponseYes, UserResponseNo, or UserResponseAll.
func PromptUserForYesNoAll(prompt string) (UserResponse, error) {
	resp, err := PromptUserForInput(prompt + " (y/a/n) ")

	if err != nil {
		return UserResponseNo, errors.WithStackTrace(err)
	}

	switch strings.ToLower(resp) {
	case "y", "yes":
		return UserResponseYes, nil
	case "a", "all":
		return UserResponseAll, nil
	default:
		return UserResponseNo, nil
	}
}

// ClearTerminal clears the terminal screen in a cross-platform compatible manner
func ClearTerminal() {
	screen.Clear()
}
