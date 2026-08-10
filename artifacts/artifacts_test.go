package artifacts

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
)

func TestResolvePythonWheel(t *testing.T) {
	t.Parallel()
	reader := openZipFixture(t, "testdata/wheel/files.json", "demo.whl")
	defer closeArchive(t, reader)

	got, err := ResolvePythonWheel(
		context.Background(),
		provides.Package{PURL: "pkg:pypi/demo-pkg@1.0"},
		reader,
	)
	if err != nil {
		t.Fatalf("ResolvePythonWheel() error = %v", err)
	}

	want := provides.SurfaceResult{Surface: provides.Surface{
		PURL: "pkg:pypi/demo-pkg@1.0",
		Provides: []provides.ProvidedName{
			artifactName("python", "demo", "module", provides.MatchPrefix, ".", "demo_pkg-1.0.dist-info/METADATA"),
			artifactName("python", "helper", "module", provides.MatchExact, "", "demo_pkg-1.0.dist-info/METADATA"),
			artifactName("python", "shared", "module", provides.MatchPrefix, ".", "demo_pkg-1.0.dist-info/METADATA"),
		},
	}}
	assertSurfaceResult(t, got, want)
}

func TestResolvePythonWheelReturnsPathsWhenMetadataExtractionFails(t *testing.T) {
	t.Parallel()
	reader := openZipFixture(t, "testdata/wheel/files.json", "demo.whl")
	defer closeArchive(t, reader)
	failing := failingExtractReader{
		Reader: reader,
		path:   "demo_pkg-1.0.dist-info/METADATA",
	}

	got, err := ResolvePythonWheel(
		context.Background(),
		provides.Package{PURL: "pkg:pypi/demo-pkg@1.0"},
		failing,
	)
	if err != nil {
		t.Fatalf("ResolvePythonWheel() error = %v", err)
	}

	want := provides.SurfaceResult{
		Surface: provides.Surface{
			PURL: "pkg:pypi/demo-pkg@1.0",
			Provides: []provides.ProvidedName{
				artifactName("python", "demo", "module", provides.MatchPrefix, ".", wheelPathSource),
				artifactName("python", "helper", "module", provides.MatchExact, "", wheelPathSource),
				artifactName("python", "shared", "module", provides.MatchPrefix, ".", wheelPathSource),
			},
		},
		Diagnostics: []provides.Diagnostic{{
			Source:  "demo_pkg-1.0.dist-info/METADATA",
			Message: "read wheel metadata: fixture extraction failure",
		}},
	}
	assertSurfaceResult(t, got, want)
}

func TestResolveJavaArchive(t *testing.T) {
	t.Parallel()
	reader := openZipFixture(t, "testdata/jar/files.json", "example.jar")
	defer closeArchive(t, reader)

	got, err := ResolveJavaArchive(
		context.Background(),
		provides.Package{PURL: "pkg:maven/org.example/demo@1.0.0"},
		reader,
	)
	if err != nil {
		t.Fatalf("ResolveJavaArchive() error = %v", err)
	}

	want := provides.SurfaceResult{Surface: provides.Surface{
		PURL: "pkg:maven/org.example/demo@1.0.0",
		Provides: []provides.ProvidedName{
			artifactName("java", "org.example.api", "package", provides.MatchPrefix, ".", jarEntriesSource),
			artifactName("java", "org.example.demo", "module", provides.MatchExact, "", "META-INF/MANIFEST.MF"),
			artifactName("java", "org.example.internal", "package", provides.MatchPrefix, ".", jarEntriesSource),
		},
	}}
	assertSurfaceResult(t, got, want)
}

func TestResolveNPMTarball(t *testing.T) {
	t.Parallel()
	reader := openTarGzipFixture(t, "testdata/npm/files.json", "example.tgz")
	defer closeArchive(t, reader)

	got, err := ResolveNPMTarball(
		context.Background(),
		provides.Package{PURL: "pkg:npm/example-package@1.0.0"},
		reader,
	)
	if err != nil {
		t.Fatalf("ResolveNPMTarball() error = %v", err)
	}

	want := provides.SurfaceResult{Surface: provides.Surface{
		PURL: "pkg:npm/example-package@1.0.0",
		Provides: []provides.ProvidedName{
			artifactName("javascript", "example-package", "package", provides.MatchExact, "", "package/package.json"),
			artifactName("javascript", "example-package/direct", "subpath", provides.MatchExact, "", "package/package.json"),
			artifactName("javascript", "example-package/features/nested/two", "subpath", provides.MatchExact, "", "package/package.json"),
			artifactName("javascript", "example-package/features/one", "subpath", provides.MatchExact, "", "package/package.json"),
			artifactName("typescript", "example-package", "package", provides.MatchExact, "", "package/package.json"),
			artifactName("typescript", "example-package/direct", "subpath", provides.MatchExact, "", "package/package.json"),
			artifactName("typescript", "example-package/features/nested/two", "subpath", provides.MatchExact, "", "package/package.json"),
			artifactName("typescript", "example-package/features/one", "subpath", provides.MatchExact, "", "package/package.json"),
		},
	}}
	assertSurfaceResult(t, got, want)
}

func TestResolveCargoCrate(t *testing.T) {
	t.Parallel()
	reader := openTarGzipFixture(t, "testdata/cargo/files.json", "example.crate")
	defer closeArchive(t, reader)

	got, err := ResolveCargoCrate(
		context.Background(),
		provides.Package{PURL: "pkg:cargo/example-package@1.0.0"},
		reader,
	)
	if err != nil {
		t.Fatalf("ResolveCargoCrate() error = %v", err)
	}

	want := provides.SurfaceResult{Surface: provides.Surface{
		PURL: "pkg:cargo/example-package@1.0.0",
		Provides: []provides.ProvidedName{
			artifactName("rust", "custom_crate", "crate", provides.MatchPrefix, "::", "example-package-1.0.0/Cargo.toml"),
		},
	}}
	assertSurfaceResult(t, got, want)
}

func TestResolveGoModule(t *testing.T) {
	t.Parallel()
	reader := openZipFixture(t, "testdata/golang/files.json", "module.zip")
	defer closeArchive(t, reader)

	got, err := ResolveGoModule(
		context.Background(),
		provides.Package{PURL: "pkg:golang/example.com/mod@v1.2.3"},
		reader,
	)
	if err != nil {
		t.Fatalf("ResolveGoModule() error = %v", err)
	}

	want := provides.SurfaceResult{Surface: provides.Surface{
		PURL: "pkg:golang/example.com/mod@v1.2.3",
		Provides: []provides.ProvidedName{
			artifactName("go", "example.com/mod", "package", provides.MatchExact, "", goModulePathsSource),
			artifactName("go", "example.com/mod/foo", "package", provides.MatchExact, "", goModulePathsSource),
			artifactName("go", "example.com/mod/internal/bar", "package", provides.MatchExact, "", goModulePathsSource),
		},
	}}
	assertSurfaceResult(t, got, want)
}

func TestJavaModuleName(t *testing.T) {
	t.Parallel()

	got, err := javaModuleName(moduleInfoFixture("org.example.explicit"))
	if err != nil {
		t.Fatalf("javaModuleName() error = %v", err)
	}
	if got != "org.example.explicit" {
		t.Fatalf("javaModuleName() = %q, want org.example.explicit", got)
	}
}

func artifactName(
	language string,
	name string,
	kind string,
	match provides.MatchMode,
	separator string,
	source string,
) provides.ProvidedName {
	return provides.ProvidedName{
		Language:  language,
		Name:      name,
		Kind:      kind,
		Match:     match,
		Separator: separator,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceArtifact,
			Source: source,
		}},
	}
}

func assertSurfaceResult(t *testing.T, got, want provides.SurfaceResult) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("surface result = %#v, want %#v", got, want)
	}
}

type failingExtractReader struct {
	archives.Reader
	path string
}

func (reader failingExtractReader) Extract(filename string) (io.ReadCloser, error) {
	if filename == reader.path {
		return nil, errors.New("fixture extraction failure")
	}
	return reader.Reader.Extract(filename)
}

//nolint:ireturn // test helper returns the production archive interface
func openZipFixture(t *testing.T, filename, archiveName string) archives.Reader {
	t.Helper()
	files := readArtifactFixture(t, filename)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range sortedFixtureNames(files) {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := archives.OpenBytes(archiveName, buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

//nolint:ireturn // test helper returns the production archive interface
func openTarGzipFixture(t *testing.T, filename, archiveName string) archives.Reader {
	t.Helper()
	files := readArtifactFixture(t, filename)
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, name := range sortedFixtureNames(files) {
		content := []byte(files[name])
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := archives.OpenBytes(archiveName, buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func readArtifactFixture(t *testing.T, filename string) map[string]string {
	t.Helper()
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var files map[string]string
	if err = json.Unmarshal(content, &files); err != nil {
		t.Fatal(err)
	}
	return files
}

func sortedFixtureNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func closeArchive(t *testing.T, reader archives.Reader) {
	t.Helper()
	if err := reader.Close(); err != nil {
		t.Errorf("close archive: %v", err)
	}
}

func moduleInfoFixture(moduleName string) []byte {
	var content []byte
	content = appendUint32(content, 0xcafebabe)
	content = appendUint16(content, 0)
	content = appendUint16(content, 53)
	content = appendUint16(content, 4)
	content = appendUTF8Constant(content, "Module")
	content = appendUTF8Constant(content, moduleName)
	content = append(content, 19)
	content = appendUint16(content, 2)
	content = appendUint16(content, 0x8000)
	content = appendUint16(content, 0)
	content = appendUint16(content, 0)
	content = appendUint16(content, 0)
	content = appendUint16(content, 0)
	content = appendUint16(content, 0)
	content = appendUint16(content, 1)
	content = appendUint16(content, 1)
	content = appendUint32(content, 2)
	content = appendUint16(content, 3)
	return content
}

func appendUTF8Constant(content []byte, value string) []byte {
	content = append(content, 1)
	content = appendUint16(content, uint16(len(value)))
	return append(content, value...)
}

func appendUint16(content []byte, value uint16) []byte {
	return append(content, byte(value>>8), byte(value))
}

func appendUint32(content []byte, value uint32) []byte {
	return append(content, byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
}
