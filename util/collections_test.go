package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReverseStringSlice tests that a slice of strings is correctly reversed by
// util.ReverseStringSlice()
func TestReverseStringSlice(t *testing.T) {
	s := []string{"a", "b", "c"}
	expected := []string{"c", "b", "a"}
	actual := ReverseStringSlice(s)
	require.Equal(t, expected, actual)
}
