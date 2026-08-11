package provides

import (
	"context"
	"fmt"
	"sort"
)

// ImportRequest describes a source import and the project dependencies that
// may provide it.
type ImportRequest struct {
	Language string
	Name     string
	Packages []Package
	Options  SurfaceOptions
}

// ImportMatch connects an import to one matching package surface.
type ImportMatch struct {
	PURL     string
	Provided ProvidedName
}

// ImportResult contains every package surface matching an import and any
// non-fatal diagnostics collected while resolving project surfaces.
type ImportResult struct {
	Language    string
	Name        string
	Matches     []ImportMatch
	Diagnostics []Diagnostic
}

// ResolveImport resolves the requested project package surfaces and returns
// every package that provides the import.
func ResolveImport(
	ctx context.Context,
	resolver SurfaceResolver,
	request ImportRequest,
) (ImportResult, error) {
	if request.Language == "" {
		return ImportResult{}, fmt.Errorf("resolve import: language is empty")
	}
	if request.Name == "" {
		return ImportResult{}, fmt.Errorf("resolve import: name is empty")
	}

	project, err := ResolveProjectSurfaces(ctx, resolver, request.Packages, request.Options)
	return MatchImport(request.Language, request.Name, project), err
}

// MatchImport returns every matching package from an already resolved project.
func MatchImport(language, name string, project ProjectSurfaceResult) ImportResult {
	matches := make(map[importMatchKey]ImportMatch)
	for _, surface := range project.Surfaces {
		for _, provided := range surface.Provides {
			if provided.Language != language || !provided.Matches(name) {
				continue
			}

			provided.Match = normalizedMatchMode(provided.Match)
			if provided.Match != MatchPrefix {
				provided.Separator = ""
			}
			key := importMatchKey{
				purl:            surface.PURL,
				language:        provided.Language,
				name:            provided.Name,
				kind:            provided.Kind,
				match:           provided.Match,
				separator:       provided.Separator,
				caseInsensitive: provided.CaseInsensitive,
			}
			if existing, ok := matches[key]; ok {
				provided.Evidence = mergeEvidence(existing.Provided.Evidence, provided.Evidence)
			} else {
				provided.Evidence = mergeEvidence(provided.Evidence)
			}
			matches[key] = ImportMatch{PURL: surface.PURL, Provided: provided}
		}
	}

	ordered := make([]ImportMatch, 0, len(matches))
	for _, match := range matches {
		ordered = append(ordered, match)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].PURL != ordered[j].PURL {
			return ordered[i].PURL < ordered[j].PURL
		}
		left := ordered[i].Provided
		right := ordered[j].Provided
		if left.Language != right.Language {
			return left.Language < right.Language
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Match != right.Match {
			return left.Match < right.Match
		}
		return left.Separator < right.Separator
	})

	return ImportResult{
		Language:    language,
		Name:        name,
		Matches:     ordered,
		Diagnostics: mergeDiagnostics(project.Diagnostics),
	}
}

type importMatchKey struct {
	purl            string
	language        string
	name            string
	kind            string
	match           MatchMode
	separator       string
	caseInsensitive bool
}
