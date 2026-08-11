package provides

import "context"

// SurfaceResolver resolves the source-level surface provided by a package.
type SurfaceResolver interface {
	ResolveSurface(ctx context.Context, pkg Package, options SurfaceOptions) (SurfaceResult, error)
}

// BindingResolver resolves dependency bindings for a project directory.
type BindingResolver interface {
	ResolveBindings(ctx context.Context, projectDir string) (BindingResult, error)
}

// SurfaceResolverFunc adapts a function to a SurfaceResolver.
type SurfaceResolverFunc func(ctx context.Context, pkg Package, options SurfaceOptions) (SurfaceResult, error)

// ResolveSurface calls f.
func (f SurfaceResolverFunc) ResolveSurface(ctx context.Context, pkg Package, options SurfaceOptions) (SurfaceResult, error) {
	return f(ctx, pkg, options)
}

// Chain returns a SurfaceResolver that queries each resolver in order and
// merges every non-empty result for a package. Later resolvers still run
// after an earlier one produces a result so callers can combine, for
// example, a curated catalog with a heuristic fallback and receive both
// mappings with their distinct evidence. A resolver that returns an error
// contributes a diagnostic and the chain continues.
func Chain(resolvers ...SurfaceResolver) SurfaceResolverFunc {
	return func(ctx context.Context, pkg Package, options SurfaceOptions) (SurfaceResult, error) {
		var results []SurfaceResult
		var diagnostics []Diagnostic
		for _, r := range resolvers {
			if r == nil {
				continue
			}
			res, err := r.ResolveSurface(ctx, pkg, options)
			// Diagnostics are collected once here; strip them from what
			// goes to MergeSurfaceResults so they are not merged twice.
			diagnostics = append(diagnostics, res.Diagnostics...)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{Source: "chain", Message: err.Error()})
				continue
			}
			if len(res.Surface.Provides) > 0 {
				results = append(results, SurfaceResult{Surface: res.Surface})
			}
		}
		merged := MergeSurfaceResults(pkg.PURL, results...)
		merged.Diagnostics = mergeDiagnostics(append(merged.Diagnostics, diagnostics...))
		return merged, nil
	}
}
