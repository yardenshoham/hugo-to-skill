package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

var kbFlatFixture = filepath.Join("..", "pkg", "site", "testdata", "sites", "kb-flat")

func TestGenerateCmd(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "fixture-kb")

	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"generate", kbFlatFixture, "--content-path", "kb", "--output", outputDir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check SKILL.md exists
	if _, err := os.Stat(filepath.Join(outputDir, "SKILL.md")); os.IsNotExist(err) {
		t.Error("SKILL.md not generated")
	}

	// Check the mirrored references tree exists
	refs, err := os.ReadDir(filepath.Join(outputDir, "references", "kb"))
	if err != nil {
		t.Fatalf("Failed to read references/kb directory: %v", err)
	}

	// _index.md + 4 pages (draft excluded)
	if len(refs) != 5 {
		t.Errorf("Expected 5 reference files, got %d", len(refs))
	}
}

func TestGenerateCmdFlagErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"missing output", []string{"generate", kbFlatFixture}},
		{"bad metadata", []string{"generate", kbFlatFixture, "--output", "ignored", "--metadata", "no-equals-sign"}},
		{"bad name", []string{"generate", kbFlatFixture, "--output", "ignored", "--name", "Bad Name!"}},
		{"missing site", []string{"generate", filepath.Join("does", "not", "exist"), "--output", "ignored"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rootCmd := newRootCmd()
			rootCmd.SetArgs(tt.args)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetOut(io.Discard)
			if err := rootCmd.Execute(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
