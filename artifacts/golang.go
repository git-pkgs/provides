package artifacts

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
)

const goModulePathsSource = "go module paths"

// ResolveGoModule reports import paths found in a Go module archive.
func ResolveGoModule(
	ctx context.Context,
	pkg provides.Package,
	reader archives.Reader,
) (provides.SurfaceResult, error) {
	identity, err := artifactIdentity(pkg, "golang")
	if err != nil {
		return provides.SurfaceResult{}, err
	}
	files, err := artifactFiles(ctx, reader)
	if err != nil {
		return provides.SurfaceResult{}, err
	}

	result := provides.SurfaceResult{Surface: provides.Surface{PURL: pkg.PURL}}
	goModPath := shortestPathWithSuffix(files, "go.mod")
	prefix := ""
	if goModPath != "" {
		prefix = pathPrefix(goModPath, "go.mod")
		content, readErr := readArtifactFile(reader, goModPath)
		if readErr != nil {
			result.Diagnostics = append(result.Diagnostics, artifactDiagnostic(
				goModPath,
				fmt.Sprintf("read go.mod: %v", readErr),
			))
		} else if modulePath := goModuleDirective(string(content)); modulePath != "" &&
			modulePath != identity.FullName() {
			result.Diagnostics = append(result.Diagnostics, artifactDiagnostic(
				goModPath,
				fmt.Sprintf("module path %q does not match PURL name %q", modulePath, identity.FullName()),
			))
		}
	}

	paths := relativeArtifactPaths(files, prefix)
	directories := make(map[string]struct{})
	for _, filename := range paths {
		if err := ctx.Err(); err != nil {
			return provides.MergeSurfaceResults(pkg.PURL, result), err
		}
		base := path.Base(filename)
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") ||
			strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") {
			continue
		}
		directory := path.Dir(filename)
		if directory == "." {
			directory = ""
		}
		if goIgnoredDirectory(directory) {
			continue
		}
		directories[directory] = struct{}{}
	}

	modulePath := identity.FullName()
	for directory := range directories {
		importPath := modulePath
		if directory != "" {
			importPath += "/" + directory
		}
		result.Surface.Provides = append(result.Surface.Provides, provides.ProvidedName{
			Language: "go",
			Name:     importPath,
			Kind:     "package",
			Match:    provides.MatchExact,
			Evidence: []provides.Evidence{{
				Method: provides.EvidenceArtifact,
				Source: goModulePathsSource,
			}},
		})
	}
	return provides.MergeSurfaceResults(pkg.PURL, result), nil
}

func goModuleDirective(content string) string {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}

func goIgnoredDirectory(directory string) bool {
	for _, component := range strings.Split(directory, "/") {
		if component == "vendor" || component == "testdata" ||
			strings.HasPrefix(component, ".") || strings.HasPrefix(component, "_") {
			return true
		}
	}
	return false
}
