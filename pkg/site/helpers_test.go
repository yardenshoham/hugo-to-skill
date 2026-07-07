package site

import "testing"

func TestHumanize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"maintenance-and-upgrade", "Maintenance and upgrade"},
		{"getting_started", "Getting started"},
		{"kb", "Kb"},
		{"", ""},
		{"àbc", "Àbc"},
	}
	for _, tt := range tests {
		if got := humanize(tt.in); got != tt.want {
			t.Errorf("humanize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFirstH1(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, want string
	}{
		{"plain", "# Title here\n\nBody.\n", "Title here"},
		{"not first line", "Some intro.\n\n# Later title\n", "Later title"},
		{"h2 only", "## Subtitle\n", ""},
		{"empty", "", ""},
		{"inside fence", "```sh\n# a comment\n```\n\n# Real title\n", "Real title"},
		{"inside tilde fence", "~~~\n# nope\n~~~\n", ""},
		{"indented code", "    # indented comment\n", ""},
		{"heading indented up to 3 spaces", "   # Indented heading\n", "Indented heading"},
	}
	for _, tt := range tests {
		if got := firstH1([]byte(tt.body)); got != tt.want {
			t.Errorf("%s: firstH1 = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFallbackName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		relPath, want string
	}{
		{"kb/some-page.md", "some-page"},
		{"docs/install/index.md", "install"},
		{"docs/_index.md", "docs"},
		{"about.md", "about"},
	}
	for _, tt := range tests {
		if got := fallbackName(tt.relPath); got != tt.want {
			t.Errorf("fallbackName(%q) = %q, want %q", tt.relPath, got, tt.want)
		}
	}
}
