package provides

import "testing"

func TestProvidedNameMatchesExactNameCaseSensitively(t *testing.T) {
	t.Parallel()

	name := ProvidedName{
		Language: "python",
		Name:     "flask",
		Kind:     "module",
	}

	if !name.Matches("flask") {
		t.Fatal("Matches(flask) = false, want true")
	}
	if name.Matches("Flask") {
		t.Fatal("Matches(Flask) = true, want case-sensitive false")
	}
	if name.Matches("flask.json") {
		t.Fatal("Matches(flask.json) = true for an exact name")
	}
}

func TestProvidedNameMatchesPrefixAtSeparatorBoundary(t *testing.T) {
	t.Parallel()

	name := ProvidedName{
		Language:  "python",
		Name:      "werkzeug",
		Kind:      "module",
		Match:     MatchPrefix,
		Separator: ".",
	}

	for _, imported := range []string{"werkzeug", "werkzeug.http"} {
		if !name.Matches(imported) {
			t.Errorf("Matches(%q) = false, want true", imported)
		}
	}
	for _, imported := range []string{"Werkzeug.http", "werkzeugx", "werkzeug/http"} {
		if name.Matches(imported) {
			t.Errorf("Matches(%q) = true, want false", imported)
		}
	}
}

func TestProvidedNameExactSubpathDoesNotMatchDescendants(t *testing.T) {
	t.Parallel()

	name := ProvidedName{
		Language: "javascript",
		Name:     "react/jsx-runtime",
		Kind:     "subpath",
		Match:    MatchExact,
	}

	if !name.Matches("react/jsx-runtime") {
		t.Fatal("Matches(react/jsx-runtime) = false, want true")
	}
	if name.Matches("react/jsx-runtime/private") {
		t.Fatal("Matches(react/jsx-runtime/private) = true for an exact export")
	}
}

func TestBindingMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		binding  Binding
		imported string
		want     bool
	}{
		{
			name:     "zero value is exact",
			binding:  Binding{Imported: "react"},
			imported: "react",
			want:     true,
		},
		{
			name:     "exact rejects subpath",
			binding:  Binding{Imported: "react", Match: MatchExact},
			imported: "react/jsx-runtime",
			want:     false,
		},
		{
			name:     "literal prefix matches suffix",
			binding:  Binding{Imported: "ui/", Match: MatchPrefix},
			imported: "ui/button",
			want:     true,
		},
		{
			name:     "literal prefix does not match trimmed name",
			binding:  Binding{Imported: "ui/", Match: MatchPrefix},
			imported: "ui",
			want:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.binding.Matches(test.imported); got != test.want {
				t.Fatalf("Binding.Matches(%q) = %v, want %v", test.imported, got, test.want)
			}
		})
	}
}
