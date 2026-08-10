package provides_test

import (
	"context"
	"errors"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/curated"
)

func TestResolveProjectSurfacesFromBareCheckoutDependencies(t *testing.T) {
	t.Parallel()

	packages := []provides.Package{
		{PURL: "pkg:pypi/PyYAML@6.0.3"},
		{PURL: "pkg:pypi/brotlipy@0.7.0"},
		{PURL: "pkg:pypi/Pillow@11.0.0"},
		{PURL: "pkg:pypi/beautifulsoup4@4.12.3"},
		{PURL: "pkg:pypi/unknown@1.0.0"},
		{PURL: "pkg:pypi/PyYAML@6.0.3"},
	}

	result, err := provides.ResolveProjectSurfaces(
		context.Background(),
		curated.Python(),
		packages,
		provides.SurfaceOptions{},
	)
	if err != nil {
		t.Fatalf("ResolveProjectSurfaces() error = %v", err)
	}
	if got := len(result.Surfaces); got != 4 {
		t.Fatalf("len(Surfaces) = %d, want 4", got)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("Diagnostics = %#v, want empty", result.Diagnostics)
	}

	wantNames := map[string]string{
		"pkg:pypi/PyYAML@6.0.3":          "yaml",
		"pkg:pypi/brotlipy@0.7.0":        "brotli",
		"pkg:pypi/Pillow@11.0.0":         "PIL",
		"pkg:pypi/beautifulsoup4@4.12.3": "bs4",
	}
	for _, surface := range result.Surfaces {
		if got := surface.Provides[0].Name; got != wantNames[surface.PURL] {
			t.Errorf("surface %q provides %q, want %q", surface.PURL, got, wantNames[surface.PURL])
		}
	}
}

func TestResolveProjectSurfacesRetainsSharedModuleProviders(t *testing.T) {
	t.Parallel()

	result, err := provides.ResolveProjectSurfaces(
		context.Background(),
		curated.Python(),
		[]provides.Package{
			{PURL: "pkg:pypi/brotlipy@0.7.0"},
			{PURL: "pkg:pypi/Brotli@1.1.0"},
		},
		provides.SurfaceOptions{},
	)
	if err != nil {
		t.Fatalf("ResolveProjectSurfaces() error = %v", err)
	}
	if got := len(result.Surfaces); got != 2 {
		t.Fatalf("len(Surfaces) = %d, want 2", got)
	}
	for _, surface := range result.Surfaces {
		if got := surface.Provides[0].Name; got != "brotli" {
			t.Errorf("surface %q provides %q, want brotli", surface.PURL, got)
		}
	}
}

func TestResolveProjectSurfacesReturnsPartialResults(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("resolver failed")
	resolver := failingResolver{err: wantErr}
	result, err := provides.ResolveProjectSurfaces(
		context.Background(),
		resolver,
		[]provides.Package{
			{PURL: "pkg:pypi/good@1.0.0"},
			{PURL: "pkg:pypi/bad@1.0.0"},
		},
		provides.SurfaceOptions{},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ResolveProjectSurfaces() error = %v, want %v", err, wantErr)
	}
	if got := len(result.Surfaces); got != 1 {
		t.Fatalf("len(Surfaces) = %d, want 1", got)
	}
	if result.Surfaces[0].PURL != "pkg:pypi/good@1.0.0" {
		t.Errorf("Surface.PURL = %q, want good package", result.Surfaces[0].PURL)
	}
}

type failingResolver struct {
	err error
}

func (r failingResolver) ResolveSurface(
	_ context.Context,
	pkg provides.Package,
	_ provides.SurfaceOptions,
) (provides.SurfaceResult, error) {
	if pkg.PURL == "pkg:pypi/bad@1.0.0" {
		return provides.SurfaceResult{}, r.err
	}
	return provides.SurfaceResult{Surface: provides.Surface{
		PURL: pkg.PURL,
		Provides: []provides.ProvidedName{{
			Language: "python",
			Name:     "good",
			Kind:     "module",
		}},
	}}, nil
}
