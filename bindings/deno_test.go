package bindings_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/bindings"
)

func TestParseDenoConfig(t *testing.T) {
	t.Parallel()
	content := readFixture(t, "testdata/deno/deno.json")

	got, err := bindings.ParseDenoConfig("deno.json", content)
	if err != nil {
		t.Fatalf("ParseDenoConfig() error = %v", err)
	}

	want := provides.BindingResult{
		Bindings: []provides.Binding{
			denoBinding("pkg:npm/%40scope/ui", "ui/", "@scope/ui/components/", provides.MatchPrefix),
			denoBinding("pkg:npm/react", "jsx", "react/jsx-runtime", provides.MatchExact),
			denoBinding("pkg:npm/react", "react", "react", provides.MatchExact),
		},
		Diagnostics: []provides.Diagnostic{{
			Source:  "deno.json",
			Message: `prefix import "bad/" has non-prefix target "npm:react@19"`,
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseDenoConfig() = %#v, want %#v", got, want)
	}
	if !got.Bindings[0].Matches("ui/button") {
		t.Fatal("prefix binding does not match ui/button")
	}
	if got.Bindings[0].Matches("ui") {
		t.Fatal("prefix binding unexpectedly matches ui")
	}
}

func TestParseDenoConfigReturnsParseError(t *testing.T) {
	t.Parallel()

	_, err := bindings.ParseDenoConfig("deno.json", []byte(`{"imports":`))
	if err == nil || !strings.Contains(err.Error(), "parse deno.json") {
		t.Fatalf("ParseDenoConfig() error = %v, want filename in parse error", err)
	}
}

func denoBinding(packageURL, imported, target string, match provides.MatchMode) provides.Binding {
	return provides.Binding{
		PURL:     packageURL,
		Imported: imported,
		Target:   target,
		Match:    match,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceManifest,
			Source: "deno.json",
		}},
	}
}
