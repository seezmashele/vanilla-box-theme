package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedFilesAreCommitted is the generator's whole safety property: what
// the tokens describe and what the repository ships are the same bytes. It
// fails whenever assets/ is edited by hand instead of through spec/tokens.json,
// which is the mistake the generator exists to make impossible.
func TestGeneratedFilesAreCommitted(t *testing.T) {
	const root = "../.."

	tk, err := loadTokens(filepath.Join(root, "spec", "tokens.json"))
	if err != nil {
		t.Fatalf("loadTokens: %v", err)
	}

	files, err := build(tk, "neutral", "sand", "rounded", "rounded", "windows")
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("the generator produced nothing")
	}

	for path, want := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("%s: %v — run go generate ./...", path, err)

			continue
		}

		if string(got) != want {
			t.Errorf("%s is out of date with spec/tokens.json — run go generate ./...", path)
		}
	}
}

// TestSquareSurfacesDropTheirCurves checks the axis the generator was built for
// before any variant depends on it: at radius zero the corner tiles must be
// plain lines, or a square variant would ship rounded artwork.
func TestSquareSurfacesDropTheirCurves(t *testing.T) {
	square := frame{
		Size: 44, Canvas: 60, Tile: 10, Radius: 0,
		Fallback: "#292929", Mask: true, HintSize: 8, HintY: 48,
	}

	svg := square.render()

	if want := `d="M0,10 L0,0 L10,0 L10,10 Z"`; !strings.Contains(svg, want) {
		t.Errorf("radius 0 should emit a plain corner tile, want %s in:\n%s", want, firstTile(svg))
	}
	if strings.Contains(svg, " C") {
		t.Error("radius 0 emitted a curve command")
	}
}

func firstTile(svg string) string {
	start := strings.Index(svg, `<g id="topleft"`)
	if start < 0 {
		return svg
	}

	return svg[start : start+strings.Index(svg[start:], "\n")]
}
