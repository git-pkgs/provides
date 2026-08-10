package bindings_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/bindings"
)

func TestParseNPMManifest(t *testing.T) {
	t.Parallel()
	content := readFixture(t, "testdata/npm/project/package.json")

	got, err := bindings.ParseNPMManifest("package.json", content)
	if err != nil {
		t.Fatalf("ParseNPMManifest() error = %v", err)
	}

	want := provides.BindingResult{Bindings: []provides.Binding{
		npmManifestBinding("pkg:npm/%40scope/toolkit", "toolkit", "@scope/toolkit"),
		npmManifestBinding("pkg:npm/react", "my-react", "react"),
		npmManifestBinding("pkg:npm/react", "react", ""),
		npmManifestBinding("pkg:npm/typescript", "typescript", ""),
		npmManifestBinding("pkg:npm/vue", "vue", ""),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNPMManifest() = %#v, want %#v", got, want)
	}
}

func TestParseNPMManifestReportsInvalidAlias(t *testing.T) {
	t.Parallel()
	content := []byte(`{"dependencies":{"alias":"npm:@scope"}}`)

	got, err := bindings.ParseNPMManifest("package.json", content)
	if err != nil {
		t.Fatalf("ParseNPMManifest() error = %v", err)
	}

	want := provides.BindingResult{Diagnostics: []provides.Diagnostic{{
		Source:  "package.json",
		Message: `invalid npm alias "npm:@scope" for dependency "alias"`,
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNPMManifest() = %#v, want %#v", got, want)
	}
}

func TestParseNPMPackage(t *testing.T) {
	t.Parallel()
	content := readFixture(t, "testdata/npm/package/package.json")

	got, err := bindings.ParseNPMPackage(
		"pkg:npm/react@19.0.0",
		"package.json",
		content,
	)
	if err != nil {
		t.Fatalf("ParseNPMPackage() error = %v", err)
	}

	want := provides.SurfaceResult{
		Surface: provides.Surface{
			PURL: "pkg:npm/react@19.0.0",
			Provides: []provides.ProvidedName{
				npmProvidedName("javascript", "react", "package"),
				npmProvidedName("javascript", "react/jsx-dev-runtime", "subpath"),
				npmProvidedName("javascript", "react/jsx-runtime", "subpath"),
				npmProvidedName("typescript", "react", "package"),
				npmProvidedName("typescript", "react/jsx-dev-runtime", "subpath"),
				npmProvidedName("typescript", "react/jsx-runtime", "subpath"),
			},
		},
		Diagnostics: []provides.Diagnostic{{
			Source:  "package.json",
			Message: `omitted export pattern "./features/*" without a package file list`,
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNPMPackage() = %#v, want %#v", got, want)
	}
}

func TestParseNPMPackageWithoutExportsProvidesRoot(t *testing.T) {
	t.Parallel()
	content := []byte(`{"name":"@scope/toolkit","main":"index.js"}`)

	got, err := bindings.ParseNPMPackage(
		"pkg:npm/%40scope/toolkit@2.0.0",
		"package.json",
		content,
	)
	if err != nil {
		t.Fatalf("ParseNPMPackage() error = %v", err)
	}

	want := provides.SurfaceResult{Surface: provides.Surface{
		PURL: "pkg:npm/%40scope/toolkit@2.0.0",
		Provides: []provides.ProvidedName{
			npmProvidedName("javascript", "@scope/toolkit", "package"),
			npmProvidedName("typescript", "@scope/toolkit", "package"),
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNPMPackage() = %#v, want %#v", got, want)
	}
}

func TestParseNPMPackageRejectsOtherPURLType(t *testing.T) {
	t.Parallel()

	_, err := bindings.ParseNPMPackage("pkg:cargo/react", "package.json", []byte(`{"name":"react"}`))
	if err == nil || !strings.Contains(err.Error(), "want npm") {
		t.Fatalf("ParseNPMPackage() error = %v, want npm type error", err)
	}
}

func npmManifestBinding(packageURL, imported, target string) provides.Binding {
	return provides.Binding{
		PURL:     packageURL,
		Imported: imported,
		Target:   target,
		Match:    provides.MatchExact,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceManifest,
			Source: "package.json",
		}},
	}
}

func npmProvidedName(language, name, kind string) provides.ProvidedName {
	return provides.ProvidedName{
		Language: language,
		Name:     name,
		Kind:     kind,
		Match:    provides.MatchExact,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceManifest,
			Source: "package.json",
		}},
	}
}
