// font_test.go — registro e medida de fontes; headless (sem janela).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTextWidthBuiltin(t *testing.T) {
	w, err := textWidth("", 16, 0)
	if err != nil || w != 0 {
		t.Fatalf("empty string: want 0, got %g (%v)", w, err)
	}
	ab, err := textWidth("ab", 16, 0)
	if err != nil {
		t.Fatal(err)
	}
	abc, _ := textWidth("abc", 16, 0)
	if !(ab > 0 && abc > ab) {
		t.Fatalf("want 0 < %g < %g", ab, abc)
	}
	big, _ := textWidth("ab", 32, 0)
	if big <= ab {
		t.Fatalf("size 32 (%g) should be wider than 16 (%g)", big, ab)
	}
}

func TestTextWidthUnknownFont(t *testing.T) {
	_, err := textWidth("hi", 16, 99)
	if err == nil || !strings.Contains(err.Error(), "unknown font 99 (not returned by load_font)") {
		t.Fatalf("want unknown font error, got %v", err)
	}
}

func TestLoadFontErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadFont(filepath.Join(dir, "nope.ttf")); err == nil {
		t.Fatal("want error for missing file")
	}
	bad := filepath.Join(dir, "bad.ttf")
	if err := os.WriteFile(bad, []byte("not a font at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFont(bad); err == nil {
		t.Fatal("want error for a file that is not a font")
	}
}

func TestFontExists(t *testing.T) {
	if !fontExists(0) {
		t.Fatal("id 0 (built-in) must exist")
	}
	if fontExists(4242) {
		t.Fatal("unknown id must not exist")
	}
}

func TestFontFaceCachedPerPair(t *testing.T) {
	fontMu.Lock()
	before := len(fontFaces)
	fontMu.Unlock()
	textWidth("x", 21, 0) // tamanho improvável de já estar no cache
	textWidth("y", 21, 0)
	fontMu.Lock()
	after := len(fontFaces)
	fontMu.Unlock()
	if after != before+1 {
		t.Fatalf("want exactly one new face, got %d -> %d", before, after)
	}
}

func TestFontConcurrent(t *testing.T) { // relevante com -race
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				textWidth("x", float64(10+j%5), 0)
			}
		}()
	}
	wg.Wait()
}
