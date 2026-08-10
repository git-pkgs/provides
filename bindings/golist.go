package bindings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/purl"
)

const goListSource = "go list -deps -json"

// ParseGoList extracts package import paths and local identifiers from the
// concatenated JSON stream produced by go list -deps -json.
func ParseGoList(content []byte) (provides.BindingResult, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	result := provides.BindingResult{}

	for {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			if err == io.EOF {
				return provides.MergeBindingResults(result), nil
			}
			return provides.MergeBindingResults(result), fmt.Errorf("parse %s: %w", goListSource, err)
		}
		if pkg.Standard || pkg.Module == nil || pkg.Module.Main {
			continue
		}

		result.Bindings = append(result.Bindings, provides.Binding{
			PURL:     purl.BuildPURLString("golang", pkg.Module.Path, pkg.Module.Version, ""),
			Imported: pkg.ImportPath,
			Local:    pkg.Name,
			Evidence: []provides.Evidence{{
				Method: provides.EvidenceResolver,
				Source: goListSource,
			}},
		})
	}
}

type goListPackage struct {
	ImportPath string        `json:"ImportPath"`
	Name       string        `json:"Name"`
	Standard   bool          `json:"Standard"`
	Module     *goListModule `json:"Module"`
}

type goListModule struct {
	Path    string        `json:"Path"`
	Version string        `json:"Version"`
	Main    bool          `json:"Main"`
	Replace *goListModule `json:"Replace"`
}
