package provides

import (
	"context"
	"testing"
)

func TestCatalogUsesVersionedEntryBeforeFallback(t *testing.T) {
	t.Parallel()

	catalog, err := NewCatalog(
		Surface{
			PURL: "pkg:pypi/example",
			Provides: []ProvidedName{{
				Language: "python",
				Name:     "fallback",
				Kind:     "module",
			}},
		},
		Surface{
			PURL: "pkg:pypi/example@2.0.0",
			Provides: []ProvidedName{{
				Language: "python",
				Name:     "versioned",
				Kind:     "module",
			}},
		},
	)
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}

	tests := []struct {
		purl string
		name string
	}{
		{purl: "pkg:pypi/example@1.0.0", name: "fallback"},
		{purl: "pkg:pypi/example@2.0.0", name: "versioned"},
	}
	for _, test := range tests {
		t.Run(test.purl, func(t *testing.T) {
			t.Parallel()

			result, resolveErr := catalog.ResolveSurface(
				context.Background(),
				Package{PURL: test.purl},
				SurfaceOptions{},
			)
			if resolveErr != nil {
				t.Fatalf("ResolveSurface() error = %v", resolveErr)
			}
			if result.Surface.PURL != test.purl {
				t.Errorf("Surface.PURL = %q, want %q", result.Surface.PURL, test.purl)
			}
			if got := result.Surface.Provides[0].Name; got != test.name {
				t.Errorf("ProvidedName.Name = %q, want %q", got, test.name)
			}
		})
	}
}

func TestCatalogRejectsInvalidPURL(t *testing.T) {
	t.Parallel()

	if _, err := NewCatalog(Surface{PURL: "not a purl"}); err == nil {
		t.Fatal("NewCatalog() error = nil, want invalid PURL error")
	}
}

func TestCatalogReturnsEmptySurfaceForUnknownPackage(t *testing.T) {
	t.Parallel()

	catalog, err := NewCatalog()
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	const packagePURL = "pkg:pypi/unknown@1.0.0"
	result, err := catalog.ResolveSurface(
		context.Background(),
		Package{PURL: packagePURL},
		SurfaceOptions{},
	)
	if err != nil {
		t.Fatalf("ResolveSurface() error = %v", err)
	}
	if result.Surface.PURL != packagePURL {
		t.Errorf("Surface.PURL = %q, want %q", result.Surface.PURL, packagePURL)
	}
	if len(result.Surface.Provides) != 0 {
		t.Errorf("Surface.Provides = %#v, want empty", result.Surface.Provides)
	}
}
