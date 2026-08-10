package artifacts

import (
	"context"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
)

const cratePathsSource = "crate paths"

// ResolveCargoCrate reports the library target provided by a Cargo crate.
func ResolveCargoCrate(
	ctx context.Context,
	pkg provides.Package,
	reader archives.Reader,
) (provides.SurfaceResult, error) {
	identity, err := artifactIdentity(pkg, "cargo")
	if err != nil {
		return provides.SurfaceResult{}, err
	}
	files, err := artifactFiles(ctx, reader)
	if err != nil {
		return provides.SurfaceResult{}, err
	}

	result := provides.SurfaceResult{Surface: provides.Surface{PURL: pkg.PURL}}
	manifestPath := shortestPathWithSuffix(files, "Cargo.toml")
	prefix := ""
	if manifestPath != "" {
		prefix = pathPrefix(manifestPath, "Cargo.toml")
	}
	paths := relativeArtifactPaths(files, prefix)
	libraryName := ""
	source := cratePathsSource
	if manifestPath != "" {
		content, readErr := readArtifactFile(reader, manifestPath)
		if readErr != nil {
			result.Diagnostics = append(result.Diagnostics, artifactDiagnostic(
				manifestPath,
				fmt.Sprintf("read Cargo manifest: %v", readErr),
			))
		} else {
			var manifest cargoPackageManifest
			if _, parseErr := toml.Decode(string(content), &manifest); parseErr != nil {
				result.Diagnostics = append(result.Diagnostics, artifactDiagnostic(
					manifestPath,
					fmt.Sprintf("parse Cargo manifest: %v", parseErr),
				))
			} else if manifest.Library != nil {
				libraryName = manifest.Library.Name
				if libraryName == "" {
					libraryName = manifest.Package.Name
				}
				source = manifestPath
			}
		}
	}

	if libraryName == "" && containsPath(paths, "src/lib.rs") {
		libraryName = identity.FullName()
	}
	if libraryName == "" {
		return provides.MergeSurfaceResults(pkg.PURL, result), nil
	}
	libraryName = strings.ReplaceAll(libraryName, "-", "_")
	result.Surface.Provides = append(result.Surface.Provides, provides.ProvidedName{
		Language:  "rust",
		Name:      libraryName,
		Kind:      "crate",
		Match:     provides.MatchPrefix,
		Separator: "::",
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceArtifact,
			Source: source,
		}},
	})
	return provides.MergeSurfaceResults(pkg.PURL, result), nil
}

type cargoPackageManifest struct {
	Package struct {
		Name string `toml:"name"`
	} `toml:"package"`
	Library *struct {
		Name string `toml:"name"`
		Path string `toml:"path"`
	} `toml:"lib"`
}

func containsPath(paths []string, wanted string) bool {
	for _, filename := range paths {
		if filename == wanted {
			return true
		}
	}
	return false
}
