package bindings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/purl"
)

const (
	javascriptLanguage = "javascript"
	typescriptLanguage = "typescript"
)

// ParseNPMManifest extracts registry dependency bindings from package.json.
// Manifest ranges do not become PURL versions. Local, Git, and URL dependencies
// are omitted because the manifest does not establish an npm registry identity.
func ParseNPMManifest(filename string, content []byte) (provides.BindingResult, error) {
	manifest, err := decodePackageJSON(filename, content)
	if err != nil {
		return provides.BindingResult{}, err
	}

	results := []provides.BindingResult{
		npmDependencyBindings(filename, manifest.Dependencies),
		npmDependencyBindings(filename, manifest.DevDependencies),
		npmDependencyBindings(filename, manifest.OptionalDependencies),
		npmDependencyBindings(filename, manifest.PeerDependencies),
	}
	return provides.MergeBindingResults(results...), nil
}

// ParseNPMPackage extracts the root and explicit exported subpaths provided by
// an npm package. Export patterns are reported as diagnostics because matching
// them requires the package file list, which this manifest-only parser lacks.
func ParseNPMPackage(
	packageURL string,
	filename string,
	content []byte,
) (provides.SurfaceResult, error) {
	return parseNPMPackage(packageURL, filename, content, nil)
}

// ParseNPMPackageFiles is like ParseNPMPackage but expands export patterns
// against package-relative artifact paths.
func ParseNPMPackageFiles(
	packageURL string,
	filename string,
	content []byte,
	files []string,
) (provides.SurfaceResult, error) {
	if files == nil {
		files = []string{}
	}
	return parseNPMPackage(packageURL, filename, content, files)
}

func parseNPMPackage(
	packageURL string,
	filename string,
	content []byte,
	files []string,
) (provides.SurfaceResult, error) {
	manifest, err := decodePackageJSON(filename, content)
	if err != nil {
		return provides.SurfaceResult{}, err
	}
	parsed, err := purl.Parse(packageURL)
	if err != nil {
		return provides.SurfaceResult{}, fmt.Errorf("parse npm package PURL %q: %w", packageURL, err)
	}
	if parsed.Type != "npm" {
		return provides.SurfaceResult{}, fmt.Errorf("parse npm package: PURL type is %q, want npm", parsed.Type)
	}

	packageName := parsed.FullName()
	result := provides.SurfaceResult{Surface: provides.Surface{PURL: packageURL}}
	if manifest.Name != "" && manifest.Name != packageName {
		result.Diagnostics = append(result.Diagnostics, provides.Diagnostic{
			Source:  filename,
			Message: fmt.Sprintf("package name %q does not match PURL name %q", manifest.Name, packageName),
		})
	}

	names, diagnostics := npmExportedNames(packageName, manifest.Exports, filename, files)
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	for _, name := range names {
		kind := "subpath"
		if name == packageName {
			kind = "package"
		}
		for _, language := range []string{javascriptLanguage, typescriptLanguage} {
			result.Surface.Provides = append(result.Surface.Provides, provides.ProvidedName{
				Language: language,
				Name:     name,
				Kind:     kind,
				Match:    provides.MatchExact,
				Evidence: []provides.Evidence{{
					Method: provides.EvidenceManifest,
					Source: filename,
				}},
			})
		}
	}

	return provides.MergeSurfaceResults(packageURL, result), nil
}

type npmPackageJSON struct {
	Name                 string            `json:"name"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	Exports              json.RawMessage   `json:"exports"`
}

func decodePackageJSON(filename string, content []byte) (npmPackageJSON, error) {
	var manifest npmPackageJSON
	if err := json.Unmarshal(content, &manifest); err != nil {
		return npmPackageJSON{}, fmt.Errorf("parse %s: %w", filename, err)
	}
	return manifest, nil
}

func npmDependencyBindings(filename string, dependencies map[string]string) provides.BindingResult {
	result := provides.BindingResult{}
	for imported, specification := range dependencies {
		packageName := imported
		target := ""
		if strings.HasPrefix(specification, "npm:") {
			var ok bool
			packageName, _, ok = parseNPMReference(specification)
			if !ok {
				result.Diagnostics = append(result.Diagnostics, provides.Diagnostic{
					Source:  filename,
					Message: fmt.Sprintf("invalid npm alias %q for dependency %q", specification, imported),
				})
				continue
			}
			if packageName != imported {
				target = packageName
			}
		} else if !isNPMRegistryDependency(specification) {
			continue
		}

		result.Bindings = append(result.Bindings, provides.Binding{
			PURL:     purl.BuildPURLString("npm", packageName, "", ""),
			Imported: imported,
			Target:   target,
			Match:    provides.MatchExact,
			Evidence: []provides.Evidence{{
				Method: provides.EvidenceManifest,
				Source: filename,
			}},
		})
	}
	return result
}

func isNPMRegistryDependency(specification string) bool {
	lower := strings.ToLower(strings.TrimSpace(specification))
	for _, prefix := range []string{
		"file:", "link:", "workspace:", "git:", "git+", "github:",
		"gitlab:", "bitbucket:", "http:", "https:", "/", "./", "../",
	} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return !strings.Contains(lower, "/") && !strings.HasSuffix(lower, ".tgz")
}

func parseNPMReference(specification string) (string, string, bool) {
	raw := strings.TrimPrefix(specification, "npm:")
	if raw == "" {
		return "", "", false
	}

	nameEnd := len(raw)
	searchFrom := 0
	if raw[0] == '@' {
		slash := strings.IndexByte(raw, '/')
		if slash < 2 || slash == len(raw)-1 {
			return "", "", false
		}
		searchFrom = slash + 1
	}
	if delimiter := strings.IndexAny(raw[searchFrom:], "@/"); delimiter >= 0 {
		nameEnd = searchFrom + delimiter
	}
	if nameEnd == 0 || (raw[0] == '@' && !strings.Contains(raw[:nameEnd], "/")) {
		return "", "", false
	}

	name := raw[:nameEnd]
	if nameEnd == len(raw) {
		return name, "", true
	}

	remainder := raw[nameEnd:]
	if remainder[0] == '/' {
		return name, remainder, true
	}
	if slash := strings.IndexByte(remainder, '/'); slash >= 0 {
		return name, remainder[slash:], true
	}
	return name, "", true
}

func npmExportedNames(
	packageName string,
	exports json.RawMessage,
	filename string,
	files []string,
) ([]string, []provides.Diagnostic) {
	if len(exports) == 0 {
		return []string{packageName}, nil
	}

	var value any
	if err := json.Unmarshal(exports, &value); err != nil {
		return nil, []provides.Diagnostic{{Source: filename, Message: "invalid package exports"}}
	}

	object, isObject := value.(map[string]any)
	if !isObject {
		if npmExportTargetAvailable(value) {
			return []string{packageName}, nil
		}
		return nil, nil
	}
	return npmObjectExportedNames(packageName, object, filename, files)
}

func npmObjectExportedNames(
	packageName string,
	object map[string]any,
	filename string,
	files []string,
) ([]string, []provides.Diagnostic) {
	for key := range object {
		if strings.HasPrefix(key, ".") {
			return npmSubpathExportedNames(packageName, object, filename, files)
		}
	}
	if npmExportTargetAvailable(object) {
		return []string{packageName}, nil
	}
	return nil, nil
}

func npmSubpathExportedNames(
	packageName string,
	object map[string]any,
	manifestFilename string,
	files []string,
) ([]string, []provides.Diagnostic) {
	names := make([]string, 0, len(object))
	diagnostics := make([]provides.Diagnostic, 0)
	for subpath, target := range object {
		if !npmExportTargetAvailable(target) {
			continue
		}
		if subpath == "." {
			names = append(names, packageName)
			continue
		}
		if !strings.HasPrefix(subpath, "./") {
			continue
		}
		if strings.Contains(subpath, "*") {
			if files == nil {
				diagnostics = append(diagnostics, provides.Diagnostic{
					Source:  manifestFilename,
					Message: fmt.Sprintf("omitted export pattern %q without a package file list", subpath),
				})
				continue
			}
			names = append(names, npmExpandedExportNames(packageName, subpath, target, files)...)
			continue
		}
		name := strings.TrimPrefix(subpath, "./")
		if name != "" {
			names = append(names, packageName+"/"+name)
		}
	}
	return names, diagnostics
}

func npmExpandedExportNames(
	packageName string,
	subpath string,
	target any,
	files []string,
) []string {
	var names []string
	for _, targetPattern := range npmExportTargetStrings(target) {
		for _, filename := range files {
			capture, matches := npmExportPatternCapture(targetPattern, filename)
			if !matches {
				continue
			}
			exported := strings.TrimPrefix(strings.ReplaceAll(subpath, "*", capture), "./")
			names = append(names, packageName+"/"+exported)
		}
	}
	return names
}

func npmExportTargetStrings(value any) []string {
	var targets []string
	switch typed := value.(type) {
	case string:
		targets = append(targets, typed)
	case []any:
		for _, item := range typed {
			targets = append(targets, npmExportTargetStrings(item)...)
		}
	case map[string]any:
		for _, item := range typed {
			targets = append(targets, npmExportTargetStrings(item)...)
		}
	}
	return targets
}

func npmExportPatternCapture(targetPattern, filename string) (string, bool) {
	targetPattern = strings.TrimPrefix(targetPattern, "./")
	stars := strings.Count(targetPattern, "*")
	if stars == 0 {
		return "", false
	}
	fixedLength := len(strings.ReplaceAll(targetPattern, "*", ""))
	variableLength := len(filename) - fixedLength
	if variableLength < 0 || variableLength%stars != 0 {
		return "", false
	}
	captureLength := variableLength / stars
	firstStar := strings.IndexByte(targetPattern, '*')
	if len(filename) < firstStar+captureLength {
		return "", false
	}
	capture := filename[firstStar : firstStar+captureLength]
	return capture, strings.ReplaceAll(targetPattern, "*", capture) == filename
}

func npmExportTargetAvailable(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed != ""
	case []any:
		for _, item := range typed {
			if npmExportTargetAvailable(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if npmExportTargetAvailable(item) {
				return true
			}
		}
	}
	return false
}
