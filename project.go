package provides

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// ProjectSurfaceResult contains the package surfaces resolved for one set of
// project dependencies and any non-fatal diagnostics.
type ProjectSurfaceResult struct {
	Surfaces    []Surface
	Diagnostics []Diagnostic
}

// ResolveProjectSurfaces resolves surfaces for caller-supplied project
// dependencies. It continues after resolver errors and returns successful
// surfaces alongside the joined error.
func ResolveProjectSurfaces(
	ctx context.Context,
	resolver SurfaceResolver,
	packages []Package,
	options SurfaceOptions,
) (ProjectSurfaceResult, error) {
	if resolver == nil {
		return ProjectSurfaceResult{}, fmt.Errorf("resolve project surfaces: nil resolver")
	}

	unique := make(map[string]Package, len(packages))
	for _, pkg := range packages {
		unique[pkg.PURL] = pkg
	}
	ordered := make([]Package, 0, len(unique))
	for _, pkg := range unique {
		ordered = append(ordered, pkg)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].PURL < ordered[j].PURL
	})

	byPURL := make(map[string][]SurfaceResult)
	diagnostics := make([]Diagnostic, 0)
	var errs []error
	for _, pkg := range ordered {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}

		result, err := resolver.ResolveSurface(ctx, pkg, options)
		diagnostics = append(diagnostics, result.Diagnostics...)
		if err != nil {
			errs = append(errs, fmt.Errorf("resolve surface %q: %w", pkg.PURL, err))
		}
		if len(result.Surface.Provides) == 0 {
			continue
		}

		resolvedPURL := result.Surface.PURL
		if resolvedPURL == "" {
			resolvedPURL = pkg.PURL
			result.Surface.PURL = resolvedPURL
		}
		byPURL[resolvedPURL] = append(byPURL[resolvedPURL], result)
	}

	purls := make([]string, 0, len(byPURL))
	for packagePURL := range byPURL {
		purls = append(purls, packagePURL)
	}
	sort.Strings(purls)

	surfaces := make([]Surface, 0, len(purls))
	for _, packagePURL := range purls {
		merged := MergeSurfaceResults(packagePURL, byPURL[packagePURL]...)
		surfaces = append(surfaces, merged.Surface)
	}

	return ProjectSurfaceResult{
		Surfaces:    surfaces,
		Diagnostics: mergeDiagnostics(diagnostics),
	}, errors.Join(errs...)
}
