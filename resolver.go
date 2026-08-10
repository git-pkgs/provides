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
