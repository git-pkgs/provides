# provides

A Go library for mapping package identities to the names used in source code. A Python distribution may provide a differently named module, a Go module may contain several package paths, and a Maven artifact may contain several Java packages. Project bindings retain dependency renames and aliases separately from the canonical package identity.

## Installation

```bash
go get github.com/git-pkgs/provides
```

## Model

`Surface` maps a versioned PURL to its provided source names. `Binding` connects a PURL to the imported and local names used by one project. An aliased binding may also retain the package-side target name. Both retain the evidence used to produce the mapping.

`ProvidedName.Name` is the exact source-visible spelling. Package-manager normalisation does not apply to it, and matching is case-sensitive. `flask` therefore does not match `Flask`.

## Matching names

Exact names match only themselves. Prefix names also match descendants separated by the configured boundary:

```go
pythonModule := provides.ProvidedName{
	Language:  "python",
	Name:      "werkzeug",
	Kind:      "module",
	Match:     provides.MatchPrefix,
	Separator: ".",
}

pythonModule.Matches("werkzeug")      // true
pythonModule.Matches("werkzeug.http") // true
pythonModule.Matches("werkzeugx")     // false
pythonModule.Matches("Werkzeug.http") // false
```

Explicit export maps can use exact matching so an exported npm subpath does not imply that deeper paths are available:

```go
export := provides.ProvidedName{
	Language: "javascript",
	Name:     "react/jsx-runtime",
	Kind:     "subpath",
	Match:    provides.MatchExact,
}

export.Matches("react/jsx-runtime")         // true
export.Matches("react/jsx-runtime/private") // false
```

## Merging results

Resolvers can find the same mapping through manifests, installed metadata, artifacts, or curated data. `MergeSurfaceResults` and `MergeBindingResults` deduplicate mappings, combine their evidence, retain conflicting mappings, and return stable ordering.

```go
const purl = "pkg:pypi/pyyaml@6.0.3"

manifest := provides.SurfaceResult{Surface: provides.Surface{
	PURL: purl,
	Provides: []provides.ProvidedName{{
		Language: "python",
		Name:     "yaml",
		Kind:     "module",
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceManifest,
			Source: "METADATA",
		}},
	}},
}}

result := provides.MergeSurfaceResults(purl, manifest)
```

Non-fatal resolver problems are returned as `Diagnostic` values beside any successful mappings.

## Curated Python surfaces

The `curated` package includes local mappings for PyYAML, `brotlipy`, Brotli, Pillow, and Beautiful Soup. `ResolveProjectSurfaces` joins caller-supplied dependency PURLs to those mappings:

```go
packages := []provides.Package{
	{PURL: "pkg:pypi/PyYAML@6.0.3"},
	{PURL: "pkg:pypi/brotlipy@0.7.0"},
	{PURL: "pkg:pypi/Pillow@11.0.0"},
}

result, err := provides.ResolveProjectSurfaces(
	context.Background(),
	curated.Python(),
	packages,
	provides.SurfaceOptions{},
)
```

This path reads no files, runs no package-manager commands, and makes no network requests. PyPI distribution names are normalised for catalog lookup, while each returned `Surface.PURL` retains the caller's spelling and version. Unknown packages are omitted without producing a diagnostic.

## Resolving an import

`ResolveImport` combines project-surface resolution with a reverse lookup. Every matching dependency is returned when an import is ambiguous:

```go
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
```

`result.Matches` contains both PURLs and the curated evidence for the `brotli` module. Supplying only the dependency declared by a project narrows the result without applying a package-name heuristic.

## Rust and Go bindings

The `bindings` package parses package-manager output supplied by the caller. It does not read files, run commands, or use the network.

`ParseCargoManifest` reads dependency, development-dependency, build-dependency, and target-specific tables from `Cargo.toml`. A renamed dependency keeps its canonical package in the PURL and exposes its source-visible crate name through `Binding.Imported`:

```go
result, err := bindings.ParseCargoManifest("Cargo.toml", cargoToml)
```

Manifest bindings use versionless PURLs because Cargo manifest versions are constraints. Path and Git dependencies are omitted when the manifest alone cannot establish a registry identity. `ParseCargoMetadata` reads the resolved root package or default workspace members and returns versioned PURLs. Its `resolve.nodes[].deps[].name` value preserves Cargo renames:

```go
result, err := bindings.ParseCargoMetadata(cargoMetadataJSON)
```

`ParseGoList` accepts the concatenated JSON stream written by `go list -deps -json`. Each non-standard dependency package becomes a binding to its module PURL. `Binding.Imported` contains the full import path and `Binding.Local` contains the declared Go package name, which may differ from the last path component:

```go
result, err := bindings.ParseGoList(goListJSON)
```

The caller controls how those byte slices are obtained. Cargo metadata is more precise than a manifest when both are available because it contains resolved versions and exact crate names.

## JavaScript and TypeScript bindings

`ParseNPMManifest` reads registry dependencies and `npm:` aliases from the dependency sections in `package.json`. Manifest ranges produce versionless PURLs. Local paths, Git sources, and URL dependencies are omitted when their npm identity cannot be established.

```go
result, err := bindings.ParseNPMManifest("package.json", packageJSON)
```

For `"my-react": "npm:react@18"`, `Binding.Imported` is `my-react`, `Binding.Target` is `react`, and the PURL is `pkg:npm/react`.

`ParseNPMPackage` reads a package root and explicit `exports` subpaths. It reports JavaScript and TypeScript names separately, with exact matching for each declared export:

```go
result, err := bindings.ParseNPMPackage(
	"pkg:npm/react@19.0.0",
	"package.json",
	packageJSON,
)
```

Export patterns are returned as diagnostics by the manifest parser. A later artifact resolver can expand them against the files shipped by the package.

`ParseDenoConfig` reads npm targets from the top-level `imports` map in `deno.json`. Keys ending in `/` become literal prefix bindings, while other keys remain exact. `Binding.Target` retains the package-side root or subpath:

```go
result, err := bindings.ParseDenoConfig("deno.json", denoJSON)
```

## Artifact surfaces

The `artifacts` package accepts an `archives.Reader` opened by the caller. It does not download artifacts or add another archive abstraction.

```go
reader, err := archives.OpenBytes("demo.whl", wheelBytes)
if err != nil {
	return err
}
defer reader.Close()

result, err := artifacts.ResolvePythonWheel(
	context.Background(),
	provides.Package{PURL: "pkg:pypi/demo@1.0.0"},
	reader,
)
```

The package contains five artifact inspectors:

- `ResolvePythonWheel` reads `Import-Name` and `Import-Namespace`, then falls back to `top_level.txt` and wheel paths when those fields are absent.
- `ResolveJavaArchive` enumerates Java packages and reads explicit or automatic module names.
- `ResolveNPMTarball` reads package roots and exports, expanding export patterns against the tarball file list.
- `ResolveCargoCrate` reads the published library target, with `src/lib.rs` as a fallback.
- `ResolveGoModule` enumerates directories containing non-test Go source files.

Entry extraction failures become diagnostics when other archive evidence can still produce names. Listing failures and invalid PURLs remain errors. The caller owns and closes the archive reader.

## Resolver interfaces

`SurfaceResolver` resolves the names provided by one package. `BindingResolver` resolves dependency bindings for a project directory. The core package defines these interfaces without running package managers or making network requests.

Further acquisition adapters are planned. The current package contains the shared types, matching rules, merge helpers, resolver interfaces, project-surface join, local binding parsers, artifact inspectors, and the built-in Python catalog used by Hyrum.
