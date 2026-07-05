package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestVersionCmd(t *testing.T) {
	t.Parallel()
	versionCmd := newVersionCmd()
	var stdout bytes.Buffer
	versionCmd.SetOut(&stdout)
	err := versionCmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// it should unmarshal well
	var version versionInfo
	err = json.Unmarshal(stdout.Bytes(), &version)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if version.Version == "" {
		t.Fatalf("Expected version to be set, got empty string")
	}
	if version.GoVersion == "" {
		t.Fatalf("Expected Go version to be set, got empty string")
	}
}
