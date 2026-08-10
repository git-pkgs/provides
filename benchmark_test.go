package provides_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/git-pkgs/provides"
)

func BenchmarkMatchImport(b *testing.B) {
	for _, packageCount := range []int{10, 100, 1_000} {
		b.Run(fmt.Sprintf("%d_packages", packageCount), func(b *testing.B) {
			project := benchmarkProject(packageCount)
			importName := fmt.Sprintf("benchmark_module_%04d.child", packageCount-1)
			var result provides.ImportResult

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result = provides.MatchImport("python", importName, project)
			}
			runtime.KeepAlive(result)
			if len(result.Matches) != 1 {
				b.Fatalf("len(Matches) = %d, want 1", len(result.Matches))
			}
		})
	}
}

func BenchmarkResolveProjectSurfaces(b *testing.B) {
	for _, packageCount := range []int{10, 100, 1_000} {
		b.Run(fmt.Sprintf("%d_packages", packageCount), func(b *testing.B) {
			catalog, packages := benchmarkCatalog(b, packageCount)
			var result provides.ProjectSurfaceResult
			var err error

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err = provides.ResolveProjectSurfaces(
					context.Background(),
					catalog,
					packages,
					provides.SurfaceOptions{},
				)
				if err != nil {
					b.Fatal(err)
				}
			}
			runtime.KeepAlive(result)
			if len(result.Surfaces) != packageCount {
				b.Fatalf("len(Surfaces) = %d, want %d", len(result.Surfaces), packageCount)
			}
		})
	}
}

func BenchmarkResolveImport(b *testing.B) {
	for _, packageCount := range []int{10, 100, 1_000} {
		b.Run(fmt.Sprintf("%d_packages", packageCount), func(b *testing.B) {
			catalog, packages := benchmarkCatalog(b, packageCount)
			request := provides.ImportRequest{
				Language: "python",
				Name:     fmt.Sprintf("benchmark_module_%04d.child", packageCount-1),
				Packages: packages,
			}
			var result provides.ImportResult
			var err error

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err = provides.ResolveImport(context.Background(), catalog, request)
				if err != nil {
					b.Fatal(err)
				}
			}
			runtime.KeepAlive(result)
			if len(result.Matches) != 1 {
				b.Fatalf("len(Matches) = %d, want 1", len(result.Matches))
			}
		})
	}
}

func benchmarkProject(packageCount int) provides.ProjectSurfaceResult {
	surfaces := make([]provides.Surface, packageCount)
	for i := range surfaces {
		surfaces[i] = benchmarkSurface(i, true)
	}
	return provides.ProjectSurfaceResult{Surfaces: surfaces}
}

func benchmarkCatalog(b *testing.B, packageCount int) (*provides.Catalog, []provides.Package) {
	b.Helper()
	surfaces := make([]provides.Surface, packageCount)
	packages := make([]provides.Package, packageCount)
	for i := range surfaces {
		surfaces[i] = benchmarkSurface(i, false)
		packages[i] = provides.Package{
			PURL: fmt.Sprintf("pkg:pypi/benchmark-package-%04d@1.0.0", i),
		}
	}
	catalog, err := provides.NewCatalog(surfaces...)
	if err != nil {
		b.Fatal(err)
	}
	return catalog, packages
}

func benchmarkSurface(index int, versioned bool) provides.Surface {
	version := ""
	if versioned {
		version = "@1.0.0"
	}
	return provides.Surface{
		PURL: fmt.Sprintf("pkg:pypi/benchmark-package-%04d%s", index, version),
		Provides: []provides.ProvidedName{{
			Language:  "python",
			Name:      fmt.Sprintf("benchmark_module_%04d", index),
			Kind:      "module",
			Match:     provides.MatchPrefix,
			Separator: ".",
			Evidence: []provides.Evidence{{
				Method: provides.EvidenceCurated,
				Source: "benchmark",
			}},
		}},
	}
}
