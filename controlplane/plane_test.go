package main

import (
	"fmt"
	"testing"
)

// Test cases to verify the implementation
func TestExtractDependencyID(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"$task0.param1", "task0"},
		{"$complex-task-id.field", "complex-task-id"},
		{"notadependency", ""},
		{"$.invalid", ""},
		{"$task0", ""},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			actual := extractDependencyID(tc.input)
			if actual != tc.expected {
				panic(fmt.Sprintf("Failed: input=%q, got=%q, want=%q", tc.input, actual, tc.expected))
			}
		})
	}
}
