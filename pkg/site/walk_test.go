package site

import (
	"testing"
	"time"
)

func TestPublishable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		fm   map[string]any
		want bool
	}{
		{"nil front matter", nil, true},
		{"plain page", map[string]any{"title": "Hi"}, true},
		{"draft", map[string]any{"draft": true}, false},
		{"draft false", map[string]any{"draft": false}, true},
		{"draft capitalized key", map[string]any{"Draft": true}, false},
		{"future publishDate string", map[string]any{"publishDate": "2100-01-02"}, false},
		{"past publishDate string", map[string]any{"publishdate": "2001-01-02"}, true},
		{"future publishDate time", map[string]any{"publishDate": time.Date(2100, 1, 2, 0, 0, 0, 0, time.UTC)}, false},
		{"past expiryDate", map[string]any{"expiryDate": "2001-01-02"}, false},
		{"future expiryDate", map[string]any{"expiryDate": "2100-01-02"}, true},
		{"headless", map[string]any{"headless": true}, false},
		{"build render never", map[string]any{"build": map[string]any{"render": "never"}}, false},
		{"build render false", map[string]any{"build": map[string]any{"render": false}}, false},
		{"build render always", map[string]any{"build": map[string]any{"render": "always"}}, true},
		{"legacy _build render never", map[string]any{"_build": map[string]any{"render": "never"}}, false},
	}
	for _, tt := range tests {
		if got := publishable(tt.fm, now); got != tt.want {
			t.Errorf("%s: publishable(%v) = %v, want %v", tt.name, tt.fm, got, tt.want)
		}
	}
}

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

func TestSplitLang(t *testing.T) {
	t.Parallel()
	w := &walker{langs: map[string]bool{"en": true, "fr": true, "zh-cn": true}}
	tests := []struct {
		stem, base, lang string
	}{
		{"about", "about", ""},
		{"about.fr", "about", "fr"},
		{"_index.zh-cn", "_index", "zh-cn"},
		{"v1.2-notes", "v1.2-notes", ""},
		{"about.de", "about.de", ""},
	}
	for _, tt := range tests {
		base, lang := w.splitLang(tt.stem)
		if base != tt.base || lang != tt.lang {
			t.Errorf("splitLang(%q) = (%q, %q), want (%q, %q)", tt.stem, base, lang, tt.base, tt.lang)
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
	}
	for _, tt := range tests {
		if got := firstH1([]byte(tt.body)); got != tt.want {
			t.Errorf("%s: firstH1 = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPageName(t *testing.T) {
	t.Parallel()
	w := &walker{langs: map[string]bool{"en": true, "fr": true}}
	tests := []struct {
		relPath, want string
	}{
		{"kb/some-page.md", "some-page"},
		{"kb/about.fr.md", "about"},
		{"docs/install/index.md", "install"},
		{"docs/_index.md", "docs"},
	}
	for _, tt := range tests {
		if got := w.pageName(tt.relPath); got != tt.want {
			t.Errorf("pageName(%q) = %q, want %q", tt.relPath, got, tt.want)
		}
	}
}
