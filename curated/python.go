// Package curated contains local package-surface catalogs.
package curated

import "github.com/git-pkgs/provides"

const pythonSource = "curated/python"

// Python returns the built-in Python distribution-to-module catalog.
func Python() *provides.Catalog {
	catalog, err := provides.NewCatalog(pythonSurfaces()...)
	if err != nil {
		panic(err)
	}
	return catalog
}

func pythonSurfaces() []provides.Surface {
	return []provides.Surface{
		pythonSurface("pyyaml", "yaml", provides.MatchPrefix),
		pythonSurface("brotlipy", "brotli", provides.MatchExact),
		pythonSurface("brotli", "brotli", provides.MatchExact),
		pythonSurface("pillow", "PIL", provides.MatchPrefix),
		pythonSurface("beautifulsoup4", "bs4", provides.MatchPrefix),
	}
}

func pythonSurface(distribution, module string, match provides.MatchMode) provides.Surface {
	separator := ""
	if match == provides.MatchPrefix {
		separator = "."
	}
	return provides.Surface{
		PURL: "pkg:pypi/" + distribution,
		Provides: []provides.ProvidedName{{
			Language:  "python",
			Name:      module,
			Kind:      "module",
			Match:     match,
			Separator: separator,
			Evidence: []provides.Evidence{{
				Method: provides.EvidenceCurated,
				Source: pythonSource,
			}},
		}},
	}
}
