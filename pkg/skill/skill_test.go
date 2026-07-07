package skill

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/yardenshoham/hugo-to-skill/pkg/site"
)

var update = flag.Bool("update", false, "regenerate golden files")

// goldenCases pins the fixture sites and generation options the golden trees
// were produced with.
var goldenCases = []struct {
	name   string
	opts   site.LoadOptions
	config Config
}{
	{
		name: "kb-flat",
		opts: site.LoadOptions{ContentPath: "kb"},
		config: Config{
			License:       "Apache-2.0",
			Compatibility: "Requires nothing",
			AllowedTools:  "Read Grep",
			Metadata:      map[string]string{"source": "fixture", "category": "kb"},
			Notes:         []string{"Fixture note."},
		},
	},
	{name: "docs-multilang"},
	{name: "toml-site"},
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func loadFixture(t *testing.T, name string, opts site.LoadOptions) *site.Site {
	t.Helper()
	s, err := site.Load(context.Background(), filepath.Join("..", "site", "testdata", "sites", name), opts, discardLogger())
	if err != nil {
		t.Fatalf("loading fixture %s: %v", name, err)
	}
	return s
}

// listFiles returns the sorted relative paths of all files under dir.
func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return files
}

func TestGoldens(t *testing.T) {
	t.Parallel()
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := loadFixture(t, tc.name, tc.opts)
			goldenDir := filepath.Join("testdata", "golden", tc.name)

			if *update {
				if err := os.RemoveAll(goldenDir); err != nil {
					t.Fatal(err)
				}
				if err := GenerateDir(context.Background(), s, goldenDir, tc.config, discardLogger()); err != nil {
					t.Fatalf("GenerateDir failed: %v", err)
				}
				return
			}

			outDir := t.TempDir()
			if err := GenerateDir(context.Background(), s, outDir, tc.config, discardLogger()); err != nil {
				t.Fatalf("GenerateDir failed: %v", err)
			}

			wantFiles := listFiles(t, goldenDir)
			gotFiles := listFiles(t, outDir)
			if strings.Join(gotFiles, "\n") != strings.Join(wantFiles, "\n") {
				t.Fatalf("file tree mismatch\ngot:\n%s\nwant:\n%s", strings.Join(gotFiles, "\n"), strings.Join(wantFiles, "\n"))
			}
			for _, file := range wantFiles {
				want, err := os.ReadFile(filepath.Join(goldenDir, file))
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(outDir, file))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s differs from golden (run with -update to regenerate)\ngot:\n%s\nwant:\n%s", file, got, want)
				}
			}
		})
	}
}

// TestGoldensSpecConformance asserts the Agent Skills spec constraints on
// every golden SKILL.md: valid name, description length, and body size.
func TestGoldensSpecConformance(t *testing.T) {
	t.Parallel()
	if *update {
		t.Skip("goldens are being regenerated")
	}
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join("testdata", "golden", tc.name, "SKILL.md"))
			if err != nil {
				t.Fatalf("reading golden SKILL.md: %v", err)
			}
			if lines := bytes.Count(b, []byte("\n")); lines > 500 {
				t.Errorf("SKILL.md has %d lines, spec guidance is 500", lines)
			}

			cfm, err := pageparser.ParseFrontMatterAndContent(bytes.NewReader(b))
			if err != nil {
				t.Fatalf("parsing SKILL.md frontmatter: %v", err)
			}
			name, _ := cfm.FrontMatter["name"].(string)
			description, _ := cfm.FrontMatter["description"].(string)
			if name == "" || len(name) > maxNameLen || !nameRegexp.MatchString(name) {
				t.Errorf("invalid skill name %q", name)
			}
			if description == "" || len(description) > maxDescriptionLen {
				t.Errorf("description missing or too long (%d chars)", len(description))
			}
		})
	}
}

// TestGoldensCopyFidelity asserts copied pages are byte-identical to their
// fixture sources.
func TestGoldensCopyFidelity(t *testing.T) {
	t.Parallel()
	if *update {
		t.Skip("goldens are being regenerated")
	}
	pairs := []struct{ golden, source string }{
		{"kb-flat/references/kb/troubleshooting-volume-detached.md", "kb-flat/content/kb/troubleshooting-volume-detached.md"},
		{"docs-multilang/references/docs/install/index.md", "docs-multilang/content/en/docs/install/index.md"},
		{"toml-site/references/guides/first-steps.md", "toml-site/content/en/guides/first-steps.md"},
	}
	for _, pair := range pairs {
		got, err := os.ReadFile(filepath.Join("testdata", "golden", filepath.FromSlash(pair.golden)))
		if err != nil {
			t.Fatalf("reading golden copy: %v", err)
		}
		want, err := os.ReadFile(filepath.Join("..", "site", "testdata", "sites", filepath.FromSlash(pair.source)))
		if err != nil {
			t.Fatalf("reading source: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is not a byte-for-byte copy of %s", pair.golden, pair.source)
		}
	}
}

func TestGenerateDirRemovesStaleReferences(t *testing.T) {
	t.Parallel()
	s := loadFixture(t, "kb-flat", site.LoadOptions{ContentPath: "kb"})
	outDir := t.TempDir()
	if err := GenerateDir(context.Background(), s, outDir, Config{}, discardLogger()); err != nil {
		t.Fatalf("GenerateDir failed: %v", err)
	}
	stale := filepath.Join(outDir, "references", "kb", "stale-page.md")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := GenerateDir(context.Background(), s, outDir, Config{}, discardLogger()); err != nil {
		t.Fatalf("GenerateDir failed on rerun: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale reference file survived regeneration")
	}
}

func TestSkillName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		title, scope, override string
		want                   string
		wantErr                bool
	}{
		{title: "Longhorn", scope: "kb", want: "longhorn-kb"},
		{title: "Longhorn", want: "longhorn"},
		{title: "Let's Encrypt", want: "lets-encrypt"},
		{title: "Docs & Guides 2.0!", want: "docs-guides-2-0"},
		{title: "Kubernetes", scope: "docs/setup", want: "kubernetes-docs-setup"},
		{title: "Anything", override: "custom-name", want: "custom-name"},
		{title: "Anything", override: "Bad Name", wantErr: true},
		{title: "Anything", override: strings.Repeat("x", 65), wantErr: true},
		{title: "!!!", wantErr: true},
		{title: strings.Repeat("very-long-title ", 10), want: strings.Trim(strings.Repeat("very-long-title-", 10)[:64], "-")},
	}
	for _, tt := range tests {
		s := &site.Site{Title: tt.title, Scope: tt.scope, Root: &site.Section{}}
		got, err := skillName(s, Config{Name: tt.override})
		if tt.wantErr {
			if err == nil {
				t.Errorf("skillName(%q, %q, %q): expected error", tt.title, tt.scope, tt.override)
			}
			continue
		}
		if err != nil {
			t.Errorf("skillName(%q, %q, %q): %v", tt.title, tt.scope, tt.override, err)
			continue
		}
		if got != tt.want {
			t.Errorf("skillName(%q, %q, %q) = %q, want %q", tt.title, tt.scope, tt.override, got, tt.want)
		}
		if len(got) > maxNameLen || !nameRegexp.MatchString(got) {
			t.Errorf("skillName(%q, %q, %q) = %q violates the spec", tt.title, tt.scope, tt.override, got)
		}
	}
}

// TestTruncate checks the description limit is enforced in bytes without ever
// splitting a multi-byte rune.
func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in       string
		maxBytes int
		want     string
	}{
		{"ascii", "abcdef", 5, "ab…"},
		{"multibyte at boundary", "aéééé", 5, "a…"}, // cutting at byte 2 would split the first é
		{"exact rune boundary", "ééééé", 7, "éé…"},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.maxBytes); got != tt.want {
			t.Errorf("%s: truncate(%q, %d) = %q, want %q", tt.name, tt.in, tt.maxBytes, got, tt.want)
		}
	}
}

// TestGenerateSkillDescriptionTruncation checks an overlong multi-byte
// description survives generation as valid UTF-8 within the spec limit.
func TestGenerateSkillDescriptionTruncation(t *testing.T) {
	t.Parallel()
	s := &site.Site{Title: "Big", Root: &site.Section{}}
	var buf bytes.Buffer
	if err := generateSkill(s, &buf, Config{Description: strings.Repeat("é", 600)}, "big"); err != nil {
		t.Fatalf("generateSkill failed: %v", err)
	}
	cfm, err := pageparser.ParseFrontMatterAndContent(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("parsing generated frontmatter: %v", err)
	}
	description, _ := cfm.FrontMatter["description"].(string)
	if len(description) > maxDescriptionLen {
		t.Errorf("description is %d bytes, want ≤ %d", len(description), maxDescriptionLen)
	}
	if !utf8.ValidString(description) {
		t.Error("description is not valid UTF-8")
	}
	if !strings.HasSuffix(description, "…") {
		t.Errorf("description does not end with an ellipsis: %q", description[len(description)-20:])
	}
}

// TestGenerateSkillSectionsOnly checks the >100-page branch of the index
// rule: top-level sections only, no per-page bullets.
func TestGenerateSkillSectionsOnly(t *testing.T) {
	t.Parallel()
	big := &site.Section{Path: "docs", Page: &site.Page{
		Path: "docs/_index.md", Title: "Documentation", Description: "All the docs",
	}}
	for i := range 120 {
		big.Pages = append(big.Pages, &site.Page{
			Path:  fmt.Sprintf("docs/page-%03d.md", i),
			Title: fmt.Sprintf("Page %03d", i),
		})
	}
	s := &site.Site{
		Title:   "Big Site",
		BaseURL: "https://big.example.com",
		Root:    &site.Section{Sections: []*site.Section{big}},
	}

	var buf bytes.Buffer
	if err := generateSkill(s, &buf, Config{}, "big-site"); err != nil {
		t.Fatalf("generateSkill failed: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "- [Documentation](references/docs/_index.md) — 121 pages, All the docs\n") {
		t.Error("missing top-level section line")
	}
	if strings.Contains(output, "page-000.md") {
		t.Error("sections-only listing should not list individual pages")
	}
	if lines := strings.Count(output, "\n"); lines > 500 {
		t.Errorf("SKILL.md has %d lines, want ≤ 500", lines)
	}
}

func TestGenerateSectionIndex(t *testing.T) {
	t.Parallel()

	sec := &site.Section{
		Path: "kb",
		Pages: []*site.Page{
			{Path: "kb/first.md", Title: "First", Description: "The first page"},
			{Path: "kb/bundle/index.md", Title: "Bundle"},
		},
		Sections: []*site.Section{
			{Path: "kb/sub", Page: &site.Page{Path: "kb/sub/_index.md", Title: "Sub", Description: "A subsection"}},
		},
	}

	t.Run("with source without trailing newline", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		if err := GenerateSectionIndex(sec, []byte("---\ntitle: KB\n---\n\nIntro."), &buf); err != nil {
			t.Fatalf("GenerateSectionIndex failed: %v", err)
		}
		output := buf.String()
		if !strings.HasPrefix(output, "---\ntitle: KB\n---\n\nIntro.\n") {
			t.Error("source not copied verbatim at the top")
		}
		if !strings.Contains(output, "\n## Pages\n\n- [First](first.md) — The first page\n- [Bundle](bundle/index.md)\n") {
			t.Errorf("pages listing wrong:\n%s", output)
		}
		if !strings.Contains(output, "\n## Subsections\n\n- [Sub](sub/_index.md) — 1 page, A subsection\n") {
			t.Errorf("subsections listing wrong:\n%s", output)
		}
	})

	t.Run("without source", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		if err := GenerateSectionIndex(sec, nil, &buf); err != nil {
			t.Fatalf("GenerateSectionIndex failed: %v", err)
		}
		if !strings.HasPrefix(buf.String(), "# Kb\n") {
			t.Errorf("minimal index missing humanized heading:\n%s", buf.String())
		}
	})
}
