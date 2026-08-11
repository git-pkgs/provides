package provides

import (
	"context"
	"errors"
	"testing"
)

func fixedResolver(name string, method EvidenceMethod) SurfaceResolverFunc {
	return func(_ context.Context, pkg Package, _ SurfaceOptions) (SurfaceResult, error) {
		return SurfaceResult{Surface: Surface{
			PURL: pkg.PURL,
			Provides: []ProvidedName{{
				Language: "python", Name: name, Kind: "module",
				Evidence: []Evidence{{Method: method, Source: "test"}},
			}},
		}}, nil
	}
}

func TestChainMergesResults(t *testing.T) {
	t.Parallel()

	chained := Chain(
		fixedResolver("yaml", EvidenceCurated),
		fixedResolver("pyyaml", EvidenceHeuristic),
	)
	res, err := chained.ResolveSurface(context.Background(), Package{PURL: "pkg:pypi/PyYAML@6.0"}, SurfaceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Surface.Provides) != 2 {
		t.Fatalf("chain should keep both distinct names: %+v", res.Surface.Provides)
	}
	for _, n := range res.Surface.Provides {
		if len(n.Evidence) != 1 {
			t.Errorf("evidence not carried through merge: %+v", n)
		}
	}
}

func TestChainDeduplicatesEvidence(t *testing.T) {
	t.Parallel()

	chained := Chain(
		fixedResolver("flask", EvidenceCurated),
		fixedResolver("flask", EvidenceHeuristic),
	)
	res, err := chained.ResolveSurface(context.Background(), Package{PURL: "pkg:pypi/flask"}, SurfaceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Surface.Provides) != 1 {
		t.Fatalf("identical names should merge: %+v", res.Surface.Provides)
	}
	if len(res.Surface.Provides[0].Evidence) != 2 {
		t.Errorf("evidence from both resolvers should be retained: %+v", res.Surface.Provides[0].Evidence)
	}
}

func TestChainContinuesAfterError(t *testing.T) {
	t.Parallel()

	failing := SurfaceResolverFunc(func(_ context.Context, _ Package, _ SurfaceOptions) (SurfaceResult, error) {
		return SurfaceResult{}, errors.New("boom")
	})
	res, err := Chain(failing, fixedResolver("ok", EvidenceHeuristic)).
		ResolveSurface(context.Background(), Package{PURL: "pkg:npm/x"}, SurfaceOptions{})
	if err != nil {
		t.Fatalf("chain should not propagate resolver error: %v", err)
	}
	if len(res.Surface.Provides) != 1 || res.Surface.Provides[0].Name != "ok" {
		t.Errorf("later resolver should still run: %+v", res.Surface)
	}
	if len(res.Diagnostics) == 0 {
		t.Error("resolver error should surface as a diagnostic")
	}
}

func TestChainDiagnosticsNotDoubleCounted(t *testing.T) {
	t.Parallel()

	noisy := SurfaceResolverFunc(func(_ context.Context, pkg Package, _ SurfaceOptions) (SurfaceResult, error) {
		return SurfaceResult{
			Surface:     Surface{PURL: pkg.PURL, Provides: []ProvidedName{{Language: "x", Name: "n"}}},
			Diagnostics: []Diagnostic{{Source: "noisy", Message: "once"}},
		}, nil
	})
	res, err := Chain(noisy).ResolveSurface(context.Background(), Package{PURL: "pkg:npm/x"}, SurfaceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range res.Diagnostics {
		if d.Source == "noisy" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("diagnostic emitted %d times, want 1: %+v", count, res.Diagnostics)
	}
}

func TestChainSkipsNilAndEmpty(t *testing.T) {
	t.Parallel()

	empty := SurfaceResolverFunc(func(_ context.Context, pkg Package, _ SurfaceOptions) (SurfaceResult, error) {
		return SurfaceResult{Surface: Surface{PURL: pkg.PURL}}, nil
	})
	res, err := Chain(nil, empty, fixedResolver("x", EvidenceHeuristic)).
		ResolveSurface(context.Background(), Package{PURL: "pkg:npm/x"}, SurfaceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Surface.Provides) != 1 {
		t.Errorf("nil/empty resolvers should not affect the result: %+v", res.Surface)
	}
}
