package provides

import (
	"fmt"
	"sort"
)

const (
	evidenceResolverRank = iota
	evidenceManifestRank
	evidenceInstalledRank
	evidenceArtifactRank
	evidenceCuratedRank
	evidenceHeuristicRank
	evidenceUnknownRank
)

// MergeSurfaceResults combines results for purl, deduplicates mappings and
// evidence, and returns deterministic output. A result for a different PURL is
// omitted and reported as a diagnostic.
func MergeSurfaceResults(purl string, results ...SurfaceResult) SurfaceResult {
	provided := make(map[providedNameKey]ProvidedName)
	diagnostics := make([]Diagnostic, 0)

	for _, result := range results {
		diagnostics = append(diagnostics, result.Diagnostics...)
		if result.Surface.PURL != "" && result.Surface.PURL != purl {
			diagnostics = append(diagnostics, Diagnostic{
				Source:  "merge",
				Message: fmt.Sprintf("ignored surface for %q while merging %q", result.Surface.PURL, purl),
			})
			continue
		}

		for _, name := range result.Surface.Provides {
			match := normalizedMatchMode(name.Match)
			separator := name.Separator
			if match != MatchPrefix {
				separator = ""
			}
			key := providedNameKey{
				language:        name.Language,
				name:            name.Name,
				kind:            name.Kind,
				match:           match,
				separator:       separator,
				caseInsensitive: name.CaseInsensitive,
			}
			current, ok := provided[key]
			if !ok {
				current = ProvidedName{
					Language:        name.Language,
					Name:            name.Name,
					Kind:            name.Kind,
					Match:           match,
					Separator:       separator,
					CaseInsensitive: name.CaseInsensitive,
				}
			}
			current.Evidence = mergeEvidence(current.Evidence, name.Evidence)
			provided[key] = current
		}
	}

	var names []ProvidedName
	if len(provided) > 0 {
		names = make([]ProvidedName, 0, len(provided))
	}
	for _, name := range provided {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i].Language != names[j].Language {
			return names[i].Language < names[j].Language
		}
		if names[i].Name != names[j].Name {
			return names[i].Name < names[j].Name
		}
		if names[i].Kind != names[j].Kind {
			return names[i].Kind < names[j].Kind
		}
		if names[i].Match != names[j].Match {
			return names[i].Match < names[j].Match
		}
		if names[i].Separator != names[j].Separator {
			return names[i].Separator < names[j].Separator
		}
		// Case-sensitive before case-insensitive so the exact-spelling
		// entry sorts first when both exist.
		return !names[i].CaseInsensitive && names[j].CaseInsensitive
	})

	return SurfaceResult{
		Surface: Surface{
			PURL:     purl,
			Provides: names,
		},
		Diagnostics: mergeDiagnostics(diagnostics),
	}
}

// MergeBindingResults combines project bindings, deduplicates mappings and
// evidence, and returns deterministic output.
func MergeBindingResults(results ...BindingResult) BindingResult {
	bindings := make(map[bindingKey]Binding)
	diagnostics := make([]Diagnostic, 0)

	for _, result := range results {
		diagnostics = append(diagnostics, result.Diagnostics...)
		for _, binding := range result.Bindings {
			match := normalizedMatchMode(binding.Match)
			key := bindingKey{
				purl:     binding.PURL,
				imported: binding.Imported,
				target:   binding.Target,
				local:    binding.Local,
				match:    match,
			}
			current, ok := bindings[key]
			if !ok {
				current = Binding{
					PURL:     binding.PURL,
					Imported: binding.Imported,
					Target:   binding.Target,
					Local:    binding.Local,
					Match:    match,
				}
			}
			current.Evidence = mergeEvidence(current.Evidence, binding.Evidence)
			bindings[key] = current
		}
	}

	var merged []Binding
	if len(bindings) > 0 {
		merged = make([]Binding, 0, len(bindings))
	}
	for _, binding := range bindings {
		merged = append(merged, binding)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].PURL != merged[j].PURL {
			return merged[i].PURL < merged[j].PURL
		}
		if merged[i].Imported != merged[j].Imported {
			return merged[i].Imported < merged[j].Imported
		}
		if merged[i].Target != merged[j].Target {
			return merged[i].Target < merged[j].Target
		}
		if merged[i].Local != merged[j].Local {
			return merged[i].Local < merged[j].Local
		}
		return merged[i].Match < merged[j].Match
	})

	return BindingResult{
		Bindings:    merged,
		Diagnostics: mergeDiagnostics(diagnostics),
	}
}

type providedNameKey struct {
	language        string
	name            string
	kind            string
	match           MatchMode
	separator       string
	caseInsensitive bool
}

type bindingKey struct {
	purl     string
	imported string
	target   string
	local    string
	match    MatchMode
}

type evidenceKey struct {
	method EvidenceMethod
	source string
}

type diagnosticKey struct {
	source  string
	message string
}

func mergeEvidence(groups ...[]Evidence) []Evidence {
	byKey := make(map[evidenceKey]Evidence)
	for _, group := range groups {
		for _, evidence := range group {
			key := evidenceKey{method: evidence.Method, source: evidence.Source}
			byKey[key] = evidence
		}
	}
	if len(byKey) == 0 {
		return nil
	}

	merged := make([]Evidence, 0, len(byKey))
	for _, evidence := range byKey {
		merged = append(merged, evidence)
	}
	sort.Slice(merged, func(i, j int) bool {
		leftRank := evidenceMethodRank(merged[i].Method)
		rightRank := evidenceMethodRank(merged[j].Method)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if merged[i].Method != merged[j].Method {
			return merged[i].Method < merged[j].Method
		}
		return merged[i].Source < merged[j].Source
	})
	return merged
}

func evidenceMethodRank(method EvidenceMethod) int {
	switch method {
	case EvidenceResolver:
		return evidenceResolverRank
	case EvidenceManifest:
		return evidenceManifestRank
	case EvidenceInstalled:
		return evidenceInstalledRank
	case EvidenceArtifact:
		return evidenceArtifactRank
	case EvidenceCurated:
		return evidenceCuratedRank
	case EvidenceHeuristic:
		return evidenceHeuristicRank
	default:
		return evidenceUnknownRank
	}
}

func mergeDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	byKey := make(map[diagnosticKey]Diagnostic)
	for _, diagnostic := range diagnostics {
		key := diagnosticKey{source: diagnostic.Source, message: diagnostic.Message}
		byKey[key] = diagnostic
	}
	if len(byKey) == 0 {
		return nil
	}

	merged := make([]Diagnostic, 0, len(byKey))
	for _, diagnostic := range byKey {
		merged = append(merged, diagnostic)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Source != merged[j].Source {
			return merged[i].Source < merged[j].Source
		}
		return merged[i].Message < merged[j].Message
	})
	return merged
}
