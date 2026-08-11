package heuristic

import (
	"context"
	"testing"

	"github.com/git-pkgs/provides"
)

func resolveOne(t *testing.T, purl string) provides.Surface {
	t.Helper()
	res, err := Resolver().ResolveSurface(context.Background(), provides.Package{PURL: purl}, provides.SurfaceOptions{})
	if err != nil {
		t.Fatalf("resolve %s: %v", purl, err)
	}
	return res.Surface
}

func firstName(t *testing.T, s provides.Surface) provides.ProvidedName {
	t.Helper()
	if len(s.Provides) == 0 {
		t.Fatalf("no provided names: %+v", s)
	}
	return s.Provides[0]
}

func TestNPM(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:npm/ws@8.17.1"))
	if n.Language != "javascript" || n.Name != "ws" || !n.Matches("ws/lib/sender") {
		t.Errorf("ws: %+v", n)
	}
	scoped := firstName(t, resolveOne(t, "pkg:npm/%40babel/core"))
	if scoped.Name != "@babel/core" || !scoped.Matches("@babel/core/lib/parse") {
		t.Errorf("scoped: %+v", scoped)
	}
	if scoped.Matches("@babel/core-utils") {
		t.Error("prefix over-match")
	}
}

func TestPyPI(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:pypi/Engine-IO-Parser@1.0"))
	if n.Name != "engine_io_parser" || !n.Matches("engine_io_parser.decode") {
		t.Errorf("pypi normalisation: %+v", n)
	}
	f := firstName(t, resolveOne(t, "pkg:pypi/Flask"))
	if !f.Matches("flask") || f.Matches("Flask") {
		t.Errorf("pypi case: %+v", f)
	}
}

func TestGo(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:golang/github.com/gin-contrib/sse"))
	if n.Name != "github.com/gin-contrib/sse" || !n.Matches("github.com/gin-contrib/sse/v2") {
		t.Errorf("go module: %+v", n)
	}
	if n.Matches("github.com/gin-contrib/sse-other") {
		t.Error("prefix over-match")
	}
}

func TestGem(t *testing.T) {
	s := resolveOne(t, "pkg:gem/active_support")
	if len(s.Provides) != 2 {
		t.Fatalf("gem should provide feature + constant: %+v", s.Provides)
	}
	var feat, konst provides.ProvidedName
	for _, n := range s.Provides {
		switch n.Kind {
		case "feature":
			feat = n
		case "constant":
			konst = n
		}
	}
	if !feat.Matches("active_support/core_ext") {
		t.Errorf("feature subpath: %+v", feat)
	}
	if konst.Name != "ActiveSupport" || !konst.Matches("ActiveSupport::Duration") {
		t.Errorf("constant: %+v", konst)
	}
}

func TestCargo(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:cargo/tokio-util"))
	if n.Name != "tokio_util" || !n.Matches("tokio_util::codec") {
		t.Errorf("cargo hyphen→underscore: %+v", n)
	}
}

func TestComposer(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:composer/guzzlehttp/guzzle"))
	if n.Language != "php" || !n.CaseInsensitive {
		t.Errorf("composer should be case-insensitive: %+v", n)
	}
	if !n.Matches(`GuzzleHttp\Client`) {
		t.Errorf("case-folded PSR-4 root: %+v", n)
	}
	if n.Matches(`App\Models\User`) {
		t.Error("unrelated namespace matched")
	}
}

func TestHex(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:hex/phoenix_html"))
	if n.Name != "PhoenixHtml" || !n.Matches("PhoenixHtml.Safe") {
		t.Errorf("hex: %+v", n)
	}
}

func TestMaven(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:maven/com.google.guava/guava"))
	if n.Name != "com.google.guava" || !n.Matches("com.google.guava.collect") {
		t.Errorf("maven group: %+v", n)
	}
}

func TestUnknownEcosystem(t *testing.T) {
	s := resolveOne(t, "pkg:conan/zlib@1.3.1")
	if len(s.Provides) != 0 {
		t.Errorf("unknown ecosystem should be empty: %+v", s)
	}
}

func TestInvalidPURL(t *testing.T) {
	res, err := Resolver().ResolveSurface(context.Background(), provides.Package{PURL: "not a purl"}, provides.SurfaceOptions{})
	if err != nil {
		t.Fatalf("invalid purl should be a diagnostic, not an error: %v", err)
	}
	if len(res.Diagnostics) == 0 {
		t.Error("expected diagnostic for invalid purl")
	}
}

func TestEvidence(t *testing.T) {
	n := firstName(t, resolveOne(t, "pkg:npm/ws"))
	if len(n.Evidence) == 0 || n.Evidence[0].Method != provides.EvidenceHeuristic {
		t.Errorf("heuristic evidence: %+v", n.Evidence)
	}
}
