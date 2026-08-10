package bindings_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/bindings"
)

func TestParseGoList(t *testing.T) {
	content := readFixture(t, "testdata/golang/list.json")

	got, err := bindings.ParseGoList(content)
	if err != nil {
		t.Fatalf("ParseGoList() error = %v", err)
	}

	want := provides.BindingResult{Bindings: []provides.Binding{
		goListBinding("example.com/dep/foo", "foo"),
		goListBinding("example.com/dep/internal/bar", "barpkg"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseGoList() = %#v, want %#v", got, want)
	}
}

func TestParseGoListReturnsPartialResult(t *testing.T) {
	content := []byte(`{
		"ImportPath":"example.com/dep/foo",
		"Name":"foo",
		"Module":{"Path":"example.com/dep", "Version":"v1.2.3"}
	}
	{`)

	got, err := bindings.ParseGoList(content)
	if err == nil || !strings.Contains(err.Error(), "parse go list -deps -json") {
		t.Fatalf("ParseGoList() error = %v, want stream parse error", err)
	}

	want := provides.BindingResult{Bindings: []provides.Binding{
		goListBinding("example.com/dep/foo", "foo"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseGoList() = %#v, want partial %#v", got, want)
	}
}

func goListBinding(imported, local string) provides.Binding {
	return provides.Binding{
		PURL:     "pkg:golang/example.com/dep@v1.2.3",
		Imported: imported,
		Local:    local,
		Match:    provides.MatchExact,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceResolver,
			Source: "go list -deps -json",
		}},
	}
}
