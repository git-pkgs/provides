package bindings_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/git-pkgs/provides"
	"github.com/git-pkgs/provides/bindings"
)

func BenchmarkParseBindings(b *testing.B) {
	tests := []struct {
		name     string
		filename string
		parse    func([]byte) (provides.BindingResult, error)
	}{
		{
			name:     "CargoManifest",
			filename: "testdata/cargo/Cargo.toml",
			parse: func(content []byte) (provides.BindingResult, error) {
				return bindings.ParseCargoManifest("Cargo.toml", content)
			},
		},
		{
			name:     "CargoMetadata",
			filename: "testdata/cargo/metadata.json",
			parse:    bindings.ParseCargoMetadata,
		},
		{
			name:     "GoList",
			filename: "testdata/golang/list.json",
			parse:    bindings.ParseGoList,
		},
		{
			name:     "NPMManifest",
			filename: "testdata/npm/project/package.json",
			parse: func(content []byte) (provides.BindingResult, error) {
				return bindings.ParseNPMManifest("package.json", content)
			},
		},
		{
			name:     "DenoConfig",
			filename: "testdata/deno/deno.json",
			parse: func(content []byte) (provides.BindingResult, error) {
				return bindings.ParseDenoConfig("deno.json", content)
			},
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			content, err := os.ReadFile(test.filename)
			if err != nil {
				b.Fatal(err)
			}
			var result provides.BindingResult

			b.ReportAllocs()
			b.SetBytes(int64(len(content)))
			b.ResetTimer()
			for b.Loop() {
				result, err = test.parse(content)
				if err != nil {
					b.Fatal(err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}
