package artifacts

import (
	"context"
	"fmt"
	"io"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
)

func BenchmarkResolveArtifacts(b *testing.B) {
	tests := []struct {
		name    string
		pkg     provides.Package
		reader  *benchmarkArchive
		resolve func(context.Context, provides.Package, archives.Reader) (provides.SurfaceResult, error)
	}{
		{
			name:    "PythonWheel",
			pkg:     provides.Package{PURL: "pkg:pypi/benchmark-package@1.0.0"},
			reader:  benchmarkPythonWheel(1_000),
			resolve: ResolvePythonWheel,
		},
		{
			name:    "JavaArchive",
			pkg:     provides.Package{PURL: "pkg:maven/org.example/benchmark@1.0.0"},
			reader:  benchmarkJavaArchive(1_000),
			resolve: ResolveJavaArchive,
		},
		{
			name:    "NPMTarball",
			pkg:     provides.Package{PURL: "pkg:npm/benchmark-package@1.0.0"},
			reader:  benchmarkNPMTarball(1_000),
			resolve: ResolveNPMTarball,
		},
		{
			name:    "CargoCrate",
			pkg:     provides.Package{PURL: "pkg:cargo/benchmark-package@1.0.0"},
			reader:  benchmarkCargoCrate(1_000),
			resolve: ResolveCargoCrate,
		},
		{
			name:    "GoModule",
			pkg:     provides.Package{PURL: "pkg:golang/example.com/mod@v1.0.0"},
			reader:  benchmarkGoModule(1_000),
			resolve: ResolveGoModule,
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			var result provides.SurfaceResult
			var err error

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err = test.resolve(context.Background(), test.pkg, test.reader)
				if err != nil {
					b.Fatal(err)
				}
			}
			runtime.KeepAlive(result)
			if len(result.Surface.Provides) == 0 {
				b.Fatal("artifact resolver returned no names")
			}
		})
	}
}

type benchmarkArchive struct {
	files   []archives.FileInfo
	content map[string]string
}

func (archive *benchmarkArchive) List() ([]archives.FileInfo, error) {
	return append([]archives.FileInfo(nil), archive.files...), nil
}

func (archive *benchmarkArchive) ListDir(string) ([]archives.FileInfo, error) {
	return nil, nil
}

func (archive *benchmarkArchive) Extract(filename string) (io.ReadCloser, error) {
	content, ok := archive.content[filename]
	if !ok {
		return nil, fmt.Errorf("benchmark file %q not found", filename)
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (archive *benchmarkArchive) Hash(string) (string, error) {
	return "", nil
}

func (archive *benchmarkArchive) Close() error {
	return nil
}

func benchmarkPythonWheel(fileCount int) *benchmarkArchive {
	files := make([]archives.FileInfo, 0, fileCount+2)
	for i := range fileCount {
		files = append(files, benchmarkFile(fmt.Sprintf("docs/file_%04d.txt", i)))
	}
	metadataPath := "benchmark_package-1.0.0.dist-info/METADATA"
	files = append(files, benchmarkFile(metadataPath), benchmarkFile("benchmark_package/__init__.py"))
	return newBenchmarkArchive(files, map[string]string{
		metadataPath: "Metadata-Version: 2.5\nImport-Name: benchmark_package\n\n",
	})
}

func benchmarkJavaArchive(fileCount int) *benchmarkArchive {
	files := make([]archives.FileInfo, 0, fileCount+1)
	for i := range fileCount {
		files = append(files, benchmarkFile(fmt.Sprintf(
			"org/example/package_%03d/Class_%04d.class",
			i%100,
			i,
		)))
	}
	manifestPath := "META-INF/MANIFEST.MF"
	files = append(files, benchmarkFile(manifestPath))
	return newBenchmarkArchive(files, map[string]string{
		manifestPath: "Manifest-Version: 1.0\nAutomatic-Module-Name: org.example.benchmark\n\n",
	})
}

func benchmarkNPMTarball(fileCount int) *benchmarkArchive {
	files := make([]archives.FileInfo, 0, fileCount+1)
	for i := range fileCount {
		files = append(files, benchmarkFile(fmt.Sprintf(
			"package/src/features/feature_%04d.js",
			i,
		)))
	}
	manifestPath := "package/package.json"
	files = append(files, benchmarkFile(manifestPath))
	return newBenchmarkArchive(files, map[string]string{
		manifestPath: `{"name":"benchmark-package","exports":{"./features/*":"./src/features/*.js"}}`,
	})
}

func benchmarkCargoCrate(fileCount int) *benchmarkArchive {
	prefix := "benchmark-package-1.0.0/"
	files := make([]archives.FileInfo, 0, fileCount+1)
	for i := range fileCount {
		files = append(files, benchmarkFile(fmt.Sprintf("%ssrc/module_%04d.rs", prefix, i)))
	}
	manifestPath := prefix + "Cargo.toml"
	files = append(files, benchmarkFile(manifestPath))
	return newBenchmarkArchive(files, map[string]string{
		manifestPath: "[package]\nname = \"benchmark-package\"\n\n[lib]\nname = \"benchmark_package\"\n",
	})
}

func benchmarkGoModule(fileCount int) *benchmarkArchive {
	prefix := "example.com/mod@v1.0.0/"
	files := make([]archives.FileInfo, 0, fileCount+1)
	for i := range fileCount {
		files = append(files, benchmarkFile(fmt.Sprintf(
			"%spackage_%03d/file_%04d.go",
			prefix,
			i/10,
			i,
		)))
	}
	manifestPath := prefix + "go.mod"
	files = append(files, benchmarkFile(manifestPath))
	return newBenchmarkArchive(files, map[string]string{
		manifestPath: "module example.com/mod\n",
	})
}

func benchmarkFile(filename string) archives.FileInfo {
	return archives.FileInfo{Path: filename, Name: path.Base(filename)}
}

func newBenchmarkArchive(
	files []archives.FileInfo,
	content map[string]string,
) *benchmarkArchive {
	for left, right := 0, len(files)-1; left < right; left, right = left+1, right-1 {
		files[left], files[right] = files[right], files[left]
	}
	return &benchmarkArchive{files: files, content: content}
}
