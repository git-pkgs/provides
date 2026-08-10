package provides_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/curated"
)

func TestResolveImportSelectsDeclaredPythonProvider(t *testing.T) {
	t.Parallel()

	result, err := provides.ResolveImport(
		context.Background(),
		curated.Python(),
		provides.ImportRequest{
			Language: "python",
			Name:     "brotli",
			Packages: []provides.Package{{PURL: "pkg:pypi/brotlipy@0.7.0"}},
		},
	)
	if err != nil {
		t.Fatalf("ResolveImport() error = %v", err)
	}
	if got := len(result.Matches); got != 1 {
		t.Fatalf("len(Matches) = %d, want 1", got)
	}
	if got := result.Matches[0].PURL; got != "pkg:pypi/brotlipy@0.7.0" {
		t.Errorf("PURL = %q, want brotlipy", got)
	}
}

func TestResolveImportRetainsAmbiguousProviders(t *testing.T) {
	t.Parallel()

	result, err := provides.ResolveImport(
		context.Background(),
		curated.Python(),
		provides.ImportRequest{
			Language: "python",
			Name:     "brotli",
			Packages: []provides.Package{
				{PURL: "pkg:pypi/brotlipy@0.7.0"},
				{PURL: "pkg:pypi/brotli@1.1.0"},
			},
		},
	)
	if err != nil {
		t.Fatalf("ResolveImport() error = %v", err)
	}

	want := []string{"pkg:pypi/brotli@1.1.0", "pkg:pypi/brotlipy@0.7.0"}
	got := make([]string, 0, len(result.Matches))
	for _, match := range result.Matches {
		got = append(got, match.PURL)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matching PURLs = %#v, want %#v", got, want)
	}
}

func TestResolveImportUsesCaseSensitivePrefixMatching(t *testing.T) {
	t.Parallel()

	request := provides.ImportRequest{
		Language: "python",
		Packages: []provides.Package{{PURL: "pkg:pypi/PyYAML@6.0.3"}},
	}

	request.Name = "yaml.loader"
	result, err := provides.ResolveImport(context.Background(), curated.Python(), request)
	if err != nil {
		t.Fatalf("ResolveImport() error = %v", err)
	}
	if got := len(result.Matches); got != 1 {
		t.Fatalf("len(Matches) = %d, want 1", got)
	}

	request.Name = "Yaml.loader"
	result, err = provides.ResolveImport(context.Background(), curated.Python(), request)
	if err != nil {
		t.Fatalf("ResolveImport() error = %v", err)
	}
	if got := len(result.Matches); got != 0 {
		t.Fatalf("len(Matches) = %d, want 0", got)
	}
}

func TestResolveImportReturnsPartialMatches(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("resolver failed")
	result, err := provides.ResolveImport(
		context.Background(),
		failingResolver{err: wantErr},
		provides.ImportRequest{
			Language: "python",
			Name:     "good",
			Packages: []provides.Package{
				{PURL: "pkg:pypi/good@1.0.0"},
				{PURL: "pkg:pypi/bad@1.0.0"},
			},
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ResolveImport() error = %v, want %v", err, wantErr)
	}
	if got := len(result.Matches); got != 1 {
		t.Fatalf("len(Matches) = %d, want 1", got)
	}
}

func TestResolveImportValidatesRequest(t *testing.T) {
	t.Parallel()

	tests := []provides.ImportRequest{
		{Name: "yaml"},
		{Language: "python"},
	}
	for _, request := range tests {
		if _, err := provides.ResolveImport(context.Background(), curated.Python(), request); err == nil {
			t.Errorf("ResolveImport(%#v) error = nil, want validation error", request)
		}
	}
}
