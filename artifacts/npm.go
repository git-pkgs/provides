package artifacts

import (
	"context"
	"fmt"

	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/bindings"
)

// ResolveNPMTarball reports package roots and exports found in an npm tarball.
func ResolveNPMTarball(
	ctx context.Context,
	pkg provides.Package,
	reader archives.Reader,
) (provides.SurfaceResult, error) {
	if _, err := artifactIdentity(pkg, "npm"); err != nil {
		return provides.SurfaceResult{}, err
	}
	files, err := artifactFiles(ctx, reader)
	if err != nil {
		return provides.SurfaceResult{}, err
	}

	result := provides.SurfaceResult{Surface: provides.Surface{PURL: pkg.PURL}}
	manifestPath := shortestPathWithSuffix(files, "package.json")
	if manifestPath == "" {
		result.Diagnostics = append(result.Diagnostics, artifactDiagnostic(
			"npm tarball",
			"package.json not found",
		))
		return provides.MergeSurfaceResults(pkg.PURL, result), nil
	}
	content, err := readArtifactFile(reader, manifestPath)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, artifactDiagnostic(
			manifestPath,
			fmt.Sprintf("read npm package manifest: %v", err),
		))
		return provides.MergeSurfaceResults(pkg.PURL, result), nil
	}

	prefix := pathPrefix(manifestPath, "package.json")
	paths := relativeArtifactPaths(files, prefix)
	parsed, err := bindings.ParseNPMPackageFiles(pkg.PURL, manifestPath, content, paths)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, artifactDiagnostic(
			manifestPath,
			fmt.Sprintf("parse npm package manifest: %v", err),
		))
		return provides.MergeSurfaceResults(pkg.PURL, result), nil
	}
	for _, name := range parsed.Surface.Provides {
		name.Evidence = []provides.Evidence{{
			Method: provides.EvidenceArtifact,
			Source: manifestPath,
		}}
		result.Surface.Provides = append(result.Surface.Provides, name)
	}
	result.Diagnostics = append(result.Diagnostics, parsed.Diagnostics...)
	return provides.MergeSurfaceResults(pkg.PURL, result), nil
}
