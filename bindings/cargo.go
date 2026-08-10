package bindings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/purl"
)

const (
	cargoMetadataSource = "cargo metadata"
)

// ParseCargoManifest extracts dependency bindings from Cargo.toml content.
// Version constraints are not used as package versions, so the returned PURLs
// are versionless. Path and Git dependencies are omitted because their registry
// identity cannot be established from the manifest alone.
func ParseCargoManifest(filename string, content []byte) (provides.BindingResult, error) {
	var manifest cargoManifest
	if _, err := toml.Decode(string(content), &manifest); err != nil {
		return provides.BindingResult{}, fmt.Errorf("parse %s: %w", filename, err)
	}

	results := []provides.BindingResult{
		cargoManifestBindings(filename, manifest.Dependencies),
		cargoManifestBindings(filename, manifest.DevDependencies),
		cargoManifestBindings(filename, manifest.BuildDependencies),
	}
	for _, target := range manifest.Targets {
		results = append(results,
			cargoManifestBindings(filename, target.Dependencies),
			cargoManifestBindings(filename, target.DevDependencies),
			cargoManifestBindings(filename, target.BuildDependencies),
		)
	}

	return provides.MergeBindingResults(results...), nil
}

// ParseCargoMetadata extracts the dependency names visible to the current
// package or workspace members from cargo metadata JSON.
func ParseCargoMetadata(content []byte) (provides.BindingResult, error) {
	var metadata cargoMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return provides.BindingResult{}, fmt.Errorf("parse %s: %w", cargoMetadataSource, err)
	}

	packages := make(map[string]cargoPackage, len(metadata.Packages))
	for _, pkg := range metadata.Packages {
		packages[pkg.ID] = pkg
	}

	nodes := make(map[string]cargoNode, len(metadata.Resolve.Nodes))
	for _, node := range metadata.Resolve.Nodes {
		nodes[node.ID] = node
	}

	result := provides.BindingResult{}
	for _, root := range cargoRoots(metadata) {
		node, ok := nodes[root]
		if !ok {
			result.Diagnostics = append(result.Diagnostics, provides.Diagnostic{
				Source:  cargoMetadataSource,
				Message: fmt.Sprintf("dependency node not found for package %q", root),
			})
			continue
		}

		for _, dependency := range node.Dependencies {
			pkg, found := packages[dependency.PackageID]
			if !found {
				result.Diagnostics = append(result.Diagnostics, provides.Diagnostic{
					Source:  cargoMetadataSource,
					Message: fmt.Sprintf("package not found for dependency %q", dependency.PackageID),
				})
				continue
			}
			result.Bindings = append(result.Bindings, provides.Binding{
				PURL:     purl.BuildPURLString("cargo", pkg.Name, pkg.Version, ""),
				Imported: dependency.Name,
				Evidence: []provides.Evidence{{
					Method: provides.EvidenceResolver,
					Source: cargoMetadataSource,
				}},
			})
		}
	}

	return provides.MergeBindingResults(result), nil
}

type cargoManifest struct {
	Dependencies      map[string]cargoDependency `toml:"dependencies"`
	DevDependencies   map[string]cargoDependency `toml:"dev-dependencies"`
	BuildDependencies map[string]cargoDependency `toml:"build-dependencies"`
	Targets           map[string]cargoTarget     `toml:"target"`
}

type cargoTarget struct {
	Dependencies      map[string]cargoDependency `toml:"dependencies"`
	DevDependencies   map[string]cargoDependency `toml:"dev-dependencies"`
	BuildDependencies map[string]cargoDependency `toml:"build-dependencies"`
}

type cargoDependency struct {
	Package string
	Path    string
	Git     string
}

func (dependency *cargoDependency) UnmarshalTOML(value any) error {
	switch typed := value.(type) {
	case string:
		return nil
	case map[string]any:
		dependency.Package, _ = typed["package"].(string)
		dependency.Path, _ = typed["path"].(string)
		dependency.Git, _ = typed["git"].(string)
		return nil
	default:
		return fmt.Errorf("unsupported Cargo dependency value %T", value)
	}
}

func cargoManifestBindings(filename string, dependencies map[string]cargoDependency) provides.BindingResult {
	result := provides.BindingResult{}
	for imported, dependency := range dependencies {
		if dependency.Path != "" || dependency.Git != "" {
			continue
		}
		packageName := dependency.Package
		if packageName == "" {
			packageName = imported
		}
		result.Bindings = append(result.Bindings, provides.Binding{
			PURL:     purl.BuildPURLString("cargo", packageName, "", ""),
			Imported: strings.ReplaceAll(imported, "-", "_"),
			Evidence: []provides.Evidence{{
				Method: provides.EvidenceManifest,
				Source: filename,
			}},
		})
	}
	return result
}

type cargoMetadata struct {
	Packages                []cargoPackage `json:"packages"`
	WorkspaceMembers        []string       `json:"workspace_members"`
	WorkspaceDefaultMembers []string       `json:"workspace_default_members"`
	Resolve                 cargoResolve   `json:"resolve"`
}

type cargoPackage struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type cargoResolve struct {
	Root  string      `json:"root"`
	Nodes []cargoNode `json:"nodes"`
}

type cargoNode struct {
	ID           string         `json:"id"`
	Dependencies []cargoNodeDep `json:"deps"`
}

type cargoNodeDep struct {
	Name      string `json:"name"`
	PackageID string `json:"pkg"`
}

func cargoRoots(metadata cargoMetadata) []string {
	if metadata.Resolve.Root != "" {
		return []string{metadata.Resolve.Root}
	}
	if len(metadata.WorkspaceDefaultMembers) > 0 {
		return metadata.WorkspaceDefaultMembers
	}
	return metadata.WorkspaceMembers
}
