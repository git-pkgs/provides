package provides

import (
	"context"
	"fmt"
	"strings"

	"github.com/git-pkgs/purl"
)

// Catalog is an immutable collection of package surfaces indexed by PURL.
// Versioned entries apply only to that version. Entries without a version are
// used as fallbacks for every version of the package.
type Catalog struct {
	surfaces map[string]Surface
}

// NewCatalog creates a curated surface catalog.
func NewCatalog(surfaces ...Surface) (*Catalog, error) {
	catalog := &Catalog{surfaces: make(map[string]Surface, len(surfaces))}
	for _, surface := range surfaces {
		key, _, err := catalogKeys(surface.PURL)
		if err != nil {
			return nil, fmt.Errorf("catalog surface %q: %w", surface.PURL, err)
		}

		normalized := Surface{
			PURL:     key,
			Provides: surface.Provides,
		}
		if existing, ok := catalog.surfaces[key]; ok {
			normalized = MergeSurfaceResults(key,
				SurfaceResult{Surface: existing},
				SurfaceResult{Surface: normalized},
			).Surface
		} else {
			normalized = MergeSurfaceResults(key, SurfaceResult{Surface: normalized}).Surface
		}
		catalog.surfaces[key] = normalized
	}
	return catalog, nil
}

// ResolveSurface implements SurfaceResolver using curated local data.
func (c *Catalog) ResolveSurface(
	ctx context.Context,
	pkg Package,
	_ SurfaceOptions,
) (SurfaceResult, error) {
	if err := ctx.Err(); err != nil {
		return SurfaceResult{}, err
	}
	if c == nil {
		return SurfaceResult{}, fmt.Errorf("resolve surface: nil catalog")
	}

	exact, versionless, err := catalogKeys(pkg.PURL)
	if err != nil {
		return SurfaceResult{}, fmt.Errorf("resolve surface %q: %w", pkg.PURL, err)
	}

	surface, ok := c.surfaces[exact]
	if !ok && versionless != exact {
		surface, ok = c.surfaces[versionless]
	}
	if !ok {
		return SurfaceResult{Surface: Surface{PURL: pkg.PURL}}, nil
	}

	return MergeSurfaceResults(pkg.PURL, SurfaceResult{Surface: Surface{
		PURL:     pkg.PURL,
		Provides: surface.Provides,
	}}), nil
}

func catalogKeys(rawPURL string) (exact, versionless string, err error) {
	parsed, err := purl.Parse(rawPURL)
	if err != nil {
		return "", "", err
	}
	if parsed.Type == "pypi" {
		parsed.Name = normalizePyPIName(parsed.Name)
	}
	exact = parsed.String()
	versionless = parsed.WithVersion("").String()
	return exact, versionless, nil
}

func normalizePyPIName(name string) string {
	var normalized strings.Builder
	normalized.Grow(len(name))
	separator := false
	for _, char := range strings.ToLower(name) {
		switch char {
		case '-', '_', '.':
			if !separator {
				normalized.WriteByte('-')
				separator = true
			}
		default:
			normalized.WriteRune(char)
			separator = false
		}
	}
	return normalized.String()
}
