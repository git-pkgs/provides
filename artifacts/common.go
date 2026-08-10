package artifacts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/purl"
)

const maxMetadataSize = 4 << 20

func artifactFiles(
	ctx context.Context,
	reader archives.Reader,
) ([]archives.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	files, err := reader.List()
	if err != nil {
		return nil, fmt.Errorf("list artifact: %w", err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func artifactIdentity(pkg provides.Package, expectedType string) (*purl.PURL, error) {
	parsed, err := purl.Parse(pkg.PURL)
	if err != nil {
		return nil, fmt.Errorf("parse package PURL %q: %w", pkg.PURL, err)
	}
	if parsed.Type != expectedType {
		return nil, fmt.Errorf("package PURL type is %q, want %s", parsed.Type, expectedType)
	}
	return parsed, nil
}

func readArtifactFile(reader archives.Reader, filename string) ([]byte, error) {
	stream, err := reader.Extract(filename)
	if err != nil {
		return nil, err
	}

	content, readErr := io.ReadAll(io.LimitReader(stream, maxMetadataSize+1))
	closeErr := stream.Close()
	if len(content) > maxMetadataSize {
		readErr = errors.Join(readErr, fmt.Errorf("file exceeds %d bytes", maxMetadataSize))
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	return content, nil
}

func shortestPathWithSuffix(files []archives.FileInfo, suffix string) string {
	match := ""
	for _, file := range files {
		if file.IsDir || (file.Path != suffix && !strings.HasSuffix(file.Path, "/"+suffix)) {
			continue
		}
		if match == "" || shorterArtifactPath(file.Path, match) {
			match = file.Path
		}
	}
	return match
}

func pathPrefix(filename, basename string) string {
	if filename == basename {
		return ""
	}
	return strings.TrimSuffix(filename, basename)
}

func relativeArtifactPaths(files []archives.FileInfo, prefix string) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir || !strings.HasPrefix(file.Path, prefix) {
			continue
		}
		filename := strings.TrimPrefix(file.Path, prefix)
		if filename == "" || filename == "." || strings.HasPrefix(filename, "../") {
			continue
		}
		cleaned := path.Clean(filename)
		if path.IsAbs(filename) || cleaned != filename {
			continue
		}
		paths = append(paths, cleaned)
	}
	sort.Strings(paths)
	return paths
}

func shorterArtifactPath(candidate, current string) bool {
	candidateDepth := strings.Count(candidate, "/")
	currentDepth := strings.Count(current, "/")
	return candidateDepth < currentDepth || (candidateDepth == currentDepth && candidate < current)
}

func artifactDiagnostic(source, message string) provides.Diagnostic {
	return provides.Diagnostic{Source: source, Message: message}
}
