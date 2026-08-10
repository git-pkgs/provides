package provides

// EvidenceMethod identifies how a package or binding mapping was obtained.
type EvidenceMethod string

const (
	// EvidenceResolver comes from package-manager resolution output.
	EvidenceResolver EvidenceMethod = "resolver"
	// EvidenceManifest comes from a manifest, lockfile, or import map.
	EvidenceManifest EvidenceMethod = "manifest"
	// EvidenceInstalled comes from installed package metadata or contents.
	EvidenceInstalled EvidenceMethod = "installed"
	// EvidenceArtifact comes from a package artifact.
	EvidenceArtifact EvidenceMethod = "artifact"
	// EvidenceCurated comes from an explicit maintained mapping.
	EvidenceCurated EvidenceMethod = "curated"
	// EvidenceHeuristic comes from an opt-in naming guess.
	EvidenceHeuristic EvidenceMethod = "heuristic"
)

// MatchMode controls how a source name matches an import name.
type MatchMode string

const (
	// MatchExact matches only the complete source name.
	MatchExact MatchMode = "exact"
	// MatchPrefix matches a source-name prefix.
	MatchPrefix MatchMode = "prefix"
)

// Evidence describes where a mapping came from.
type Evidence struct {
	Method EvidenceMethod
	Source string
}

// Package identifies a package whose source-level surface should be resolved.
type Package struct {
	PURL string
}

// Surface contains the source-level names provided by a package.
type Surface struct {
	PURL     string
	Provides []ProvidedName
}

// ProvidedName is a source-level name supplied by a package.
type ProvidedName struct {
	Language  string
	Name      string
	Kind      string
	Match     MatchMode
	Separator string
	Evidence  []Evidence
}

// Binding connects a package to the name used by one project.
type Binding struct {
	PURL     string
	Imported string
	Target   string
	Local    string
	Match    MatchMode
	Evidence []Evidence
}

// Diagnostic records a non-fatal resolver problem.
type Diagnostic struct {
	Source  string
	Message string
}

// SurfaceOptions controls package-surface resolution.
type SurfaceOptions struct {
	IncludeHeuristics bool
}

// SurfaceResult contains a package surface and any non-fatal diagnostics.
type SurfaceResult struct {
	Surface     Surface
	Diagnostics []Diagnostic
}

// BindingResult contains project bindings and any non-fatal diagnostics.
type BindingResult struct {
	Bindings    []Binding
	Diagnostics []Diagnostic
}
