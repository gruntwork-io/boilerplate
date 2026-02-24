// Package prompt provides interactive user prompting.
package prompt

// UserResponse represents the user's response to a yes/no/all prompt
type UserResponse string

const (
	UserResponseYes UserResponse = "yes"
	UserResponseNo  UserResponse = "no"
	UserResponseAll UserResponse = "all"
)
