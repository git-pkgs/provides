// Package heuristic provides a SurfaceResolver that maps a package identity
// to its conventional source-level name using per-ecosystem naming rules
// alone. It reads no files, runs no commands, and makes no network requests.
//
// The mappings cover the common case where a package's importable name is a
// mechanical transform of its registry name: an npm package `ws` provides
// module `ws`, a PyPI distribution `Engine-IO-Parser` provides module
// `engine_io_parser`, a Ruby gem `active_support` provides constant
// `ActiveSupport`. Packages whose importable name is unrelated to their
// registry name (PyYAML → yaml, Pillow → PIL) need curated data or an
// artifact resolver; chain this resolver after one of those so the
// convention fills gaps the authoritative source did not cover.
//
// Every returned ProvidedName carries EvidenceHeuristic so downstream code
// can distinguish a naming-convention guess from a verified mapping.
package heuristic

import (
	"context"
	"strings"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/purl"
)

const source = "heuristic"

// Resolver returns a SurfaceResolver that derives conventional source names
// from a package's PURL type and name. Ecosystems without a registered
// convention resolve to an empty surface with no diagnostic.
func Resolver() provides.SurfaceResolverFunc {
	return resolve
}

func resolve(ctx context.Context, pkg provides.Package, _ provides.SurfaceOptions) (provides.SurfaceResult, error) {
	if err := ctx.Err(); err != nil {
		return provides.SurfaceResult{}, err
	}
	p, err := purl.Parse(pkg.PURL)
	if err != nil {
		return provides.SurfaceResult{
			Diagnostics: []provides.Diagnostic{{Source: source, Message: err.Error()}},
		}, nil
	}
	fn, ok := conventions[p.Type]
	if !ok {
		return provides.SurfaceResult{Surface: provides.Surface{PURL: pkg.PURL}}, nil
	}
	return provides.SurfaceResult{
		Surface: provides.Surface{PURL: pkg.PURL, Provides: fn(p)},
	}, nil
}

// conventions maps a PURL type to the source names its packages
// conventionally provide.
var conventions = map[string]func(*purl.PURL) []provides.ProvidedName{
	"npm": func(p *purl.PURL) []provides.ProvidedName {
		// Bare specifier and any subpath under it: `ws`, `ws/lib/sender`.
		name := p.Name
		if p.Namespace != "" {
			name = p.Namespace + "/" + p.Name
		}
		return []provides.ProvidedName{prefix("javascript", name, "module", "/", false)}
	},
	"pypi": func(p *purl.PURL) []provides.ProvidedName {
		// PEP 503 treats -, _, . as equivalent and the registry name is
		// case-insensitive; module names are lowercase with underscores.
		return []provides.ProvidedName{
			prefix("python", strings.ToLower(underscore(p.Name)), "module", ".", false),
		}
	},
	"golang": func(p *purl.PURL) []provides.ProvidedName {
		// Import paths are the module path or a package under it.
		module := p.Name
		if p.Namespace != "" {
			module = p.Namespace + "/" + p.Name
		}
		return []provides.ProvidedName{prefix("go", module, "package", "/", false)}
	},
	"gem": func(p *purl.PURL) []provides.ProvidedName {
		return []provides.ProvidedName{
			// require 'gem' or 'gem/sub'
			prefix("ruby", p.Name, "feature", "/", false),
			// Bundler-autoloaded top-level constant.
			prefix("ruby", camelize(p.Name), "constant", "::", false),
		}
	},
	"cargo": func(p *purl.PURL) []provides.ProvidedName {
		// Crate identifiers replace hyphens with underscores.
		return []provides.ProvidedName{prefix("rust", underscore(p.Name), "crate", "::", false)}
	},
	"composer": func(p *purl.PURL) []provides.ProvidedName {
		// PSR-4 root is conventionally the vendor segment titlecased. PHP
		// namespace resolution is case-insensitive so the guess need only
		// match after case folding.
		vendor := p.Namespace
		if vendor == "" {
			vendor = p.Name
		}
		return []provides.ProvidedName{prefix("php", camelize(vendor), "namespace", `\`, true)}
	},
	"hex": func(p *purl.PURL) []provides.ProvidedName {
		return []provides.ProvidedName{prefix("elixir", camelize(p.Name), "module", ".", false)}
	},
	"maven": func(p *purl.PURL) []provides.ProvidedName {
		// Java packages conventionally follow the reversed-domain group ID.
		return []provides.ProvidedName{prefix("java", p.Namespace, "package", ".", false)}
	},
}

func prefix(lang, name, kind, sep string, ci bool) provides.ProvidedName {
	return provides.ProvidedName{
		Language:        lang,
		Name:            name,
		Kind:            kind,
		Match:           provides.MatchPrefix,
		Separator:       sep,
		CaseInsensitive: ci,
		Evidence:        []provides.Evidence{{Method: provides.EvidenceHeuristic, Source: source}},
	}
}

// underscore replaces hyphens with underscores.
func underscore(s string) string { return strings.ReplaceAll(s, "-", "_") }

// camelize turns a hyphen/underscore-separated name into UpperCamelCase.
func camelize(s string) string {
	var b strings.Builder
	up := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c == '-' {
			up = true
			continue
		}
		if up && 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		up = false
		b.WriteByte(c)
	}
	return b.String()
}
