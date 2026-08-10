package provides

import "strings"

// Matches reports whether imported is covered by the provided name. Matching
// is case-sensitive because Name retains its exact source-visible spelling.
func (name ProvidedName) Matches(imported string) bool {
	if imported == name.Name {
		return true
	}
	return normalizedMatchMode(name.Match) == MatchPrefix &&
		name.Separator != "" &&
		strings.HasPrefix(imported, name.Name+name.Separator)
}

// Matches reports whether imported is covered by the project binding. Prefix
// bindings retain their complete literal prefix, including any trailing slash.
func (binding Binding) Matches(imported string) bool {
	if normalizedMatchMode(binding.Match) == MatchPrefix {
		return strings.HasPrefix(imported, binding.Imported)
	}
	return imported == binding.Imported
}

func normalizedMatchMode(mode MatchMode) MatchMode {
	if mode == "" {
		return MatchExact
	}
	return mode
}
