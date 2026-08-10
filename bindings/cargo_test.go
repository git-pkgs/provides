package bindings_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/bindings"
)

func TestParseCargoManifest(t *testing.T) {
	content := readFixture(t, "testdata/cargo/Cargo.toml")

	got, err := bindings.ParseCargoManifest("Cargo.toml", content)
	if err != nil {
		t.Fatalf("ParseCargoManifest() error = %v", err)
	}

	want := provides.BindingResult{Bindings: []provides.Binding{
		cargoManifestBinding("pkg:cargo/actual-package", "renamed_crate"),
		cargoManifestBinding("pkg:cargo/dev-package", "dev_alias"),
		cargoManifestBinding("pkg:cargo/hyphenated-package", "hyphenated_package"),
		cargoManifestBinding("pkg:cargo/serde", "serde"),
		cargoManifestBinding("pkg:cargo/unix-package", "unix_alias"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCargoManifest() = %#v, want %#v", got, want)
	}
}

func TestParseCargoManifestReturnsParseError(t *testing.T) {
	_, err := bindings.ParseCargoManifest("Cargo.toml", []byte("[dependencies"))
	if err == nil || !strings.Contains(err.Error(), "parse Cargo.toml") {
		t.Fatalf("ParseCargoManifest() error = %v, want filename in parse error", err)
	}
}

func TestParseCargoMetadata(t *testing.T) {
	content := readFixture(t, "testdata/cargo/metadata.json")

	got, err := bindings.ParseCargoMetadata(content)
	if err != nil {
		t.Fatalf("ParseCargoMetadata() error = %v", err)
	}

	want := provides.BindingResult{Bindings: []provides.Binding{
		cargoMetadataBinding("pkg:cargo/actual-package@2.1.0", "renamed_crate"),
		cargoMetadataBinding("pkg:cargo/normal-dep@1.4.0", "normal_dep"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCargoMetadata() = %#v, want %#v", got, want)
	}
}

func TestParseCargoMetadataUsesDefaultWorkspaceMembers(t *testing.T) {
	content := []byte(`{
		"packages": [
			{"id":"member-a", "name":"member-a", "version":"0.1.0"},
			{"id":"member-b", "name":"member-b", "version":"0.1.0"},
			{"id":"shared", "name":"shared", "version":"1.0.0"}
		],
		"workspace_members": ["member-a", "member-b"],
		"workspace_default_members": ["member-b"],
		"resolve": {
			"root": null,
			"nodes": [
				{"id":"member-a", "deps":[{"name":"ignored", "pkg":"shared"}]},
				{"id":"member-b", "deps":[{"name":"shared_alias", "pkg":"shared"}]}
			]
		}
	}`)

	got, err := bindings.ParseCargoMetadata(content)
	if err != nil {
		t.Fatalf("ParseCargoMetadata() error = %v", err)
	}

	want := provides.BindingResult{Bindings: []provides.Binding{
		cargoMetadataBinding("pkg:cargo/shared@1.0.0", "shared_alias"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCargoMetadata() = %#v, want %#v", got, want)
	}
}

func TestParseCargoMetadataReportsMissingNodesAndPackages(t *testing.T) {
	content := []byte(`{
		"packages": [],
		"workspace_members": ["missing-node", "member"],
		"resolve": {
			"root": null,
			"nodes": [
				{"id":"member", "deps":[{"name":"missing", "pkg":"missing-package"}]}
			]
		}
	}`)

	got, err := bindings.ParseCargoMetadata(content)
	if err != nil {
		t.Fatalf("ParseCargoMetadata() error = %v", err)
	}

	want := provides.BindingResult{Diagnostics: []provides.Diagnostic{
		{Source: "cargo metadata", Message: `dependency node not found for package "missing-node"`},
		{Source: "cargo metadata", Message: `package not found for dependency "missing-package"`},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCargoMetadata() = %#v, want %#v", got, want)
	}
}

func cargoManifestBinding(packageURL, imported string) provides.Binding {
	return provides.Binding{
		PURL:     packageURL,
		Imported: imported,
		Match:    provides.MatchExact,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceManifest,
			Source: "Cargo.toml",
		}},
	}
}

func cargoMetadataBinding(packageURL, imported string) provides.Binding {
	return provides.Binding{
		PURL:     packageURL,
		Imported: imported,
		Match:    provides.MatchExact,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceResolver,
			Source: "cargo metadata",
		}},
	}
}

func readFixture(t *testing.T, filename string) []byte {
	t.Helper()
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	return content
}
