package util

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/gruntwork-io/boilerplate/errors"
)

var BRIGHT_GREEN = color.New(color.FgHiGreen, color.Bold)

// Prompt the user for text in the CLI. Returns the text entered by the user.
func PromptUserForInput(prompt string) (string, error) {
	BRIGHT_GREEN.Print(fmt.Sprintf("%s: ", prompt))
	reader := bufio.NewReader(os.Stdin)

	text, err := reader.ReadString('\n')
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return strings.TrimSpace(text), nil
}

// Prompt the user for a yes/no response and return true if they entered yes.
func PromptUserForYesNo(prompt string) (bool, error) {
	resp, err := PromptUserForInput(fmt.Sprintf("%s (y/n) ", prompt))

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

func ClearTerminal() {
	print("\033[H\033[2J")
}
