package bindings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/purl"
)

// ParseDenoConfig extracts npm package bindings from the imports map in a
// deno.json file. Other target schemes and relative targets are left to their
// owning resolvers.
func ParseDenoConfig(filename string, content []byte) (provides.BindingResult, error) {
	var config struct {
		Imports map[string]string `json:"imports"`
	}
	if err := json.Unmarshal(content, &config); err != nil {
		return provides.BindingResult{}, fmt.Errorf("parse %s: %w", filename, err)
	}

	result := provides.BindingResult{}
	for imported, target := range config.Imports {
		if !strings.HasPrefix(target, "npm:") {
			continue
		}
		packageName, subpath, ok := parseNPMReference(target)
		if !ok {
			result.Diagnostics = append(result.Diagnostics, provides.Diagnostic{
				Source:  filename,
				Message: fmt.Sprintf("invalid npm import-map target %q for %q", target, imported),
			})
			continue
		}

		match := provides.MatchExact
		if strings.HasSuffix(imported, "/") {
			if !strings.HasSuffix(subpath, "/") {
				result.Diagnostics = append(result.Diagnostics, provides.Diagnostic{
					Source:  filename,
					Message: fmt.Sprintf("prefix import %q has non-prefix target %q", imported, target),
				})
				continue
			}
			match = provides.MatchPrefix
		}

		result.Bindings = append(result.Bindings, provides.Binding{
			PURL:     purl.BuildPURLString("npm", packageName, "", ""),
			Imported: imported,
			Target:   packageName + subpath,
			Match:    match,
			Evidence: []provides.Evidence{{
				Method: provides.EvidenceManifest,
				Source: filename,
			}},
		})
	}

	return provides.MergeBindingResults(result), nil
}
