package artifacts

import (
	"context"
	"fmt"
	"net/mail"
	"path"
	"strings"
	"unicode"

	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
)

const wheelPathSource = "wheel paths"

// ResolvePythonWheel reports Python import names found in a wheel.
func ResolvePythonWheel(
	ctx context.Context,
	pkg provides.Package,
	reader archives.Reader,
) (provides.SurfaceResult, error) {
	if _, err := artifactIdentity(pkg, "pypi"); err != nil {
		return provides.SurfaceResult{}, err
	}
	files, err := artifactFiles(ctx, reader)
	if err != nil {
		return provides.SurfaceResult{}, err
	}

	result := provides.SurfaceResult{Surface: provides.Surface{PURL: pkg.PURL}}
	paths := relativeArtifactPaths(files, "")
	metadataFound := false
	for _, file := range files {
		if file.IsDir || !strings.HasSuffix(file.Path, ".dist-info/METADATA") {
			continue
		}
		found, names, diagnostics := pythonMetadataNames(reader, file.Path, paths)
		metadataFound = metadataFound || found
		result.Surface.Provides = append(result.Surface.Provides, names...)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}

	if !metadataFound {
		for _, file := range files {
			if file.IsDir || !strings.HasSuffix(file.Path, ".dist-info/top_level.txt") {
				continue
			}
			names, diagnostics := pythonTopLevelNames(reader, file.Path, paths)
			result.Surface.Provides = append(result.Surface.Provides, names...)
			result.Diagnostics = append(result.Diagnostics, diagnostics...)
		}
		for _, candidate := range pythonPathNames(paths) {
			result.Surface.Provides = append(result.Surface.Provides,
				pythonProvidedName(candidate.name, candidate.match, wheelPathSource),
			)
		}
	}

	return provides.MergeSurfaceResults(pkg.PURL, result), nil
}

type pythonName struct {
	name  string
	match provides.MatchMode
}

func pythonMetadataNames(
	reader archives.Reader,
	filename string,
	paths []string,
) (bool, []provides.ProvidedName, []provides.Diagnostic) {
	content, err := readArtifactFile(reader, filename)
	if err != nil {
		return false, nil, []provides.Diagnostic{artifactDiagnostic(
			filename,
			fmt.Sprintf("read wheel metadata: %v", err),
		)}
	}
	message, err := mail.ReadMessage(strings.NewReader(string(content)))
	if err != nil {
		return false, nil, []provides.Diagnostic{artifactDiagnostic(
			filename,
			fmt.Sprintf("parse wheel metadata: %v", err),
		)}
	}

	importNames := message.Header["Import-Name"]
	namespaceNames := message.Header["Import-Namespace"]
	found := len(importNames) > 0 || len(namespaceNames) > 0
	names := make([]provides.ProvidedName, 0, len(importNames)+len(namespaceNames))
	diagnostics := make([]provides.Diagnostic, 0)
	seenImports := make(map[string]struct{}, len(importNames))
	for _, value := range importNames {
		name := pythonMetadataName(value)
		if name == "" {
			continue
		}
		if !isPythonDottedName(name) {
			diagnostics = append(diagnostics, artifactDiagnostic(
				filename,
				fmt.Sprintf("ignored invalid Import-Name %q", name),
			))
			continue
		}
		seenImports[name] = struct{}{}
		names = append(names, pythonProvidedName(name, pythonNameMatch(paths, name), filename))
	}
	for _, value := range namespaceNames {
		name := pythonMetadataName(value)
		if !isPythonDottedName(name) {
			diagnostics = append(diagnostics, artifactDiagnostic(
				filename,
				fmt.Sprintf("ignored invalid Import-Namespace %q", name),
			))
			continue
		}
		if _, exists := seenImports[name]; exists {
			diagnostics = append(diagnostics, artifactDiagnostic(
				filename,
				fmt.Sprintf("import name %q is both exclusive and a namespace", name),
			))
		}
		names = append(names, pythonProvidedName(name, provides.MatchPrefix, filename))
	}
	return found, names, diagnostics
}

func pythonTopLevelNames(
	reader archives.Reader,
	filename string,
	paths []string,
) ([]provides.ProvidedName, []provides.Diagnostic) {
	content, err := readArtifactFile(reader, filename)
	if err != nil {
		return nil, []provides.Diagnostic{artifactDiagnostic(
			filename,
			fmt.Sprintf("read wheel top-level names: %v", err),
		)}
	}

	var names []provides.ProvidedName
	var diagnostics []provides.Diagnostic
	for _, line := range strings.Split(string(content), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if !isPythonDottedName(name) {
			diagnostics = append(diagnostics, artifactDiagnostic(
				filename,
				fmt.Sprintf("ignored invalid top-level import %q", name),
			))
			continue
		}
		names = append(names, pythonProvidedName(name, pythonNameMatch(paths, name), filename))
	}
	return names, diagnostics
}

func pythonMetadataName(value string) string {
	name, _, _ := strings.Cut(value, ";")
	return strings.TrimSpace(name)
}

func pythonPathNames(paths []string) []pythonName {
	byName := make(map[string]provides.MatchMode)
	for _, filename := range paths {
		filename = wheelInstallPath(filename)
		if filename == "" || strings.Contains(filename, ".dist-info/") {
			continue
		}
		parts := strings.Split(filename, "/")
		if len(parts) == 1 {
			name := pythonModuleFilename(parts[0])
			if isPythonIdentifier(name) {
				byName[name] = provides.MatchExact
			}
			continue
		}
		if !isPythonIdentifier(parts[0]) || pythonModuleFilename(parts[len(parts)-1]) == "" {
			continue
		}
		byName[parts[0]] = provides.MatchPrefix
	}

	names := make([]pythonName, 0, len(byName))
	for name, match := range byName {
		names = append(names, pythonName{name: name, match: match})
	}
	return names
}

func wheelInstallPath(filename string) string {
	parts := strings.Split(filename, "/")
	if len(parts) >= 3 && strings.HasSuffix(parts[0], ".data") &&
		(parts[1] == "purelib" || parts[1] == "platlib") {
		return strings.Join(parts[2:], "/")
	}
	return filename
}

func pythonModuleFilename(filename string) string {
	base := path.Base(filename)
	if base == "__init__.py" || base == "__init__.pyi" {
		return ""
	}
	switch {
	case strings.HasSuffix(base, ".pyi"):
		return strings.TrimSuffix(base, ".pyi")
	case strings.HasSuffix(base, ".py"):
		return strings.TrimSuffix(base, ".py")
	case strings.HasSuffix(base, ".pyd") || strings.HasSuffix(base, ".so"):
		name, _, _ := strings.Cut(base, ".")
		return name
	default:
		return ""
	}
}

func pythonNameMatch(paths []string, name string) provides.MatchMode {
	modulePath := strings.ReplaceAll(name, ".", "/")
	for _, filename := range paths {
		filename = wheelInstallPath(filename)
		if strings.HasPrefix(filename, modulePath+"/") {
			return provides.MatchPrefix
		}
	}
	return provides.MatchExact
}

func pythonProvidedName(name string, match provides.MatchMode, source string) provides.ProvidedName {
	separator := ""
	if match == provides.MatchPrefix {
		separator = "."
	}
	return provides.ProvidedName{
		Language:  "python",
		Name:      name,
		Kind:      "module",
		Match:     match,
		Separator: separator,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceArtifact,
			Source: source,
		}},
	}
}

func isPythonDottedName(name string) bool {
	if name == "" {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !isPythonIdentifier(part) {
			return false
		}
	}
	return true
}

func isPythonIdentifier(name string) bool {
	for index, char := range name {
		if index == 0 {
			if char != '_' && !unicode.IsLetter(char) {
				return false
			}
			continue
		}
		if char != '_' && !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return false
		}
	}
	return name != ""
}
