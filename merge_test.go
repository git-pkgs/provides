package provides

import (
	"context"
	"reflect"
	"testing"
)

func TestMergeSurfaceResultsDeduplicatesMappingsAndEvidence(t *testing.T) {
	t.Parallel()

	purl := "pkg:pypi/pyyaml@6.0.3"
	manifestEvidence := Evidence{Method: EvidenceManifest, Source: "METADATA"}
	artifactEvidence := Evidence{Method: EvidenceArtifact, Source: "PyYAML-6.0.3.whl"}

	result := MergeSurfaceResults(purl,
		SurfaceResult{Surface: Surface{
			PURL: purl,
			Provides: []ProvidedName{{
				Language: "python",
				Name:     "yaml",
				Kind:     "module",
				Evidence: []Evidence{manifestEvidence},
			}},
		}},
		SurfaceResult{Surface: Surface{
			PURL: purl,
			Provides: []ProvidedName{{
				Language: "python",
				Name:     "yaml",
				Kind:     "module",
				Evidence: []Evidence{artifactEvidence, manifestEvidence},
			}},
		}},
	)

	want := SurfaceResult{Surface: Surface{
		PURL: purl,
		Provides: []ProvidedName{{
			Language: "python",
			Name:     "yaml",
			Kind:     "module",
			Match:    MatchExact,
			Evidence: []Evidence{manifestEvidence, artifactEvidence},
		}},
	}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("MergeSurfaceResults() = %#v, want %#v", result, want)
	}
}

func TestMergeSurfaceResultsRetainsConflictingNames(t *testing.T) {
	t.Parallel()

	purl := "pkg:pypi/pyyaml@6.0.3"
	result := MergeSurfaceResults(purl,
		SurfaceResult{Surface: Surface{Provides: []ProvidedName{{
			Language: "python",
			Name:     "yaml",
			Kind:     "module",
			Evidence: []Evidence{{Method: EvidenceArtifact, Source: "wheel"}},
		}}}},
		SurfaceResult{Surface: Surface{Provides: []ProvidedName{{
			Language: "python",
			Name:     "pyyaml",
			Kind:     "module",
			Evidence: []Evidence{{Method: EvidenceCurated, Source: "brief"}},
		}}}},
	)

	if got := len(result.Surface.Provides); got != 2 {
		t.Fatalf("len(Provides) = %d, want 2", got)
	}
	if result.Surface.Provides[0].Name != "pyyaml" || result.Surface.Provides[1].Name != "yaml" {
		t.Fatalf("Provides = %#v, want conflicting names retained in stable order", result.Surface.Provides)
	}
}

func TestMergeSurfaceResultsPreservesPartialDiagnostics(t *testing.T) {
	t.Parallel()

	purl := "pkg:maven/org.example/example@1.0.0"
	diagnostic := Diagnostic{Source: "module-info", Message: "invalid module descriptor"}
	result := MergeSurfaceResults(purl,
		SurfaceResult{Surface: Surface{Provides: []ProvidedName{{
			Language: "java",
			Name:     "org.example.api",
			Kind:     "package",
		}}}},
		SurfaceResult{Diagnostics: []Diagnostic{diagnostic, diagnostic}},
	)

	if got := len(result.Surface.Provides); got != 1 {
		t.Fatalf("len(Provides) = %d, want 1", got)
	}
	if !reflect.DeepEqual(result.Diagnostics, []Diagnostic{diagnostic}) {
		t.Fatalf("Diagnostics = %#v, want %#v", result.Diagnostics, []Diagnostic{diagnostic})
	}
}

func TestMergeSurfaceResultsHasDeterministicOrder(t *testing.T) {
	t.Parallel()

	purl := "pkg:generic/example@1.0.0"
	first := SurfaceResult{
		Surface: Surface{Provides: []ProvidedName{
			{Language: "rust", Name: "zed", Kind: "crate"},
			{Language: "go", Name: "example.com/mod/z", Kind: "package"},
		}},
		Diagnostics: []Diagnostic{{Source: "z", Message: "later"}},
	}
	second := SurfaceResult{
		Surface: Surface{Provides: []ProvidedName{
			{Language: "go", Name: "example.com/mod/a", Kind: "package"},
			{Language: "go", Name: "example.com/mod/a", Kind: "module"},
		}},
		Diagnostics: []Diagnostic{{Source: "a", Message: "earlier"}},
	}

	forward := MergeSurfaceResults(purl, first, second)
	reverse := MergeSurfaceResults(purl, second, first)
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("input order changed result:\nforward: %#v\nreverse: %#v", forward, reverse)
	}
}

func TestMergeBindingResultsDeduplicatesAndOrders(t *testing.T) {
	t.Parallel()

	resolverEvidence := Evidence{Method: EvidenceResolver, Source: "cargo metadata"}
	manifestEvidence := Evidence{Method: EvidenceManifest, Source: "Cargo.toml"}
	result := MergeBindingResults(
		BindingResult{Bindings: []Binding{{
			PURL:     "pkg:cargo/foo@1.0.0",
			Imported: "bar",
			Evidence: []Evidence{manifestEvidence},
		}}},
		BindingResult{Bindings: []Binding{
			{
				PURL:     "pkg:cargo/foo@1.0.0",
				Imported: "bar",
				Evidence: []Evidence{resolverEvidence, manifestEvidence},
			},
			{
				PURL:     "pkg:cargo/foo@1.0.0",
				Imported: "bar",
				Local:    "baz",
			},
		}},
	)

	want := []Binding{
		{
			PURL:     "pkg:cargo/foo@1.0.0",
			Imported: "bar",
			Match:    MatchExact,
			Evidence: []Evidence{resolverEvidence, manifestEvidence},
		},
		{
			PURL:     "pkg:cargo/foo@1.0.0",
			Imported: "bar",
			Local:    "baz",
			Match:    MatchExact,
		},
	}
	if !reflect.DeepEqual(result.Bindings, want) {
		t.Fatalf("Bindings = %#v, want %#v", result.Bindings, want)
	}
}

type surfaceResolverFunc func(context.Context, Package, SurfaceOptions) (SurfaceResult, error)

func (f surfaceResolverFunc) ResolveSurface(
	ctx context.Context,
	pkg Package,
	options SurfaceOptions,
) (SurfaceResult, error) {
	return f(ctx, pkg, options)
}

type bindingResolverFunc func(context.Context, string) (BindingResult, error)

func (f bindingResolverFunc) ResolveBindings(ctx context.Context, projectDir string) (BindingResult, error) {
	return f(ctx, projectDir)
}

var (
	_ SurfaceResolver = surfaceResolverFunc(nil)
	_ BindingResolver = bindingResolverFunc(nil)
)
