package cmd

import (
	"testing"
)

func TestRootCmd(t *testing.T) {
	t.Parallel()
	rootCmd := newRootCmd()
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}
