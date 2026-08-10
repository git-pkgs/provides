package curated_test

import (
	"context"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/curated"
)

func TestPythonCatalog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		purl       string
		module     string
		descendant string
		prefix     bool
	}{
		{purl: "pkg:pypi/PyYAML@6.0.3", module: "yaml", descendant: "yaml.loader", prefix: true},
		{purl: "pkg:pypi/brotlipy@0.7.0", module: "brotli"},
		{purl: "pkg:pypi/Brotli@1.1.0", module: "brotli"},
		{purl: "pkg:pypi/Pillow@11.0.0", module: "PIL", descendant: "PIL.Image", prefix: true},
		{purl: "pkg:pypi/beautifulsoup4@4.12.3", module: "bs4", descendant: "bs4.element", prefix: true},
	}

	catalog := curated.Python()
	for _, test := range tests {
		t.Run(test.purl, func(t *testing.T) {
			t.Parallel()

			result, err := catalog.ResolveSurface(
				context.Background(),
				provides.Package{PURL: test.purl},
				provides.SurfaceOptions{},
			)
			if err != nil {
				t.Fatalf("ResolveSurface() error = %v", err)
			}
			if got := len(result.Surface.Provides); got != 1 {
				t.Fatalf("len(Provides) = %d, want 1", got)
			}
			name := result.Surface.Provides[0]
			if name.Name != test.module {
				t.Errorf("Name = %q, want %q", name.Name, test.module)
			}
			if !name.Matches(test.module) {
				t.Errorf("Matches(%q) = false, want true", test.module)
			}
			if test.descendant != "" && name.Matches(test.descendant) != test.prefix {
				t.Errorf("Matches(%q) = %v, want %v", test.descendant, name.Matches(test.descendant), test.prefix)
			}
			if len(name.Evidence) != 1 || name.Evidence[0].Method != provides.EvidenceCurated {
				t.Errorf("Evidence = %#v, want one curated entry", name.Evidence)
			}
		})
	}
}
