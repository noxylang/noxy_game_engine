// font_test.go — textWidth é headless: mede com a face, sem janela.
package main

import (
	"sync"
	"testing"
)

func TestTextWidth(t *testing.T) {
	if w := textWidth("", 16); w != 0 {
		t.Fatalf("empty string: want 0, got %g", w)
	}
	ab := textWidth("ab", 16)
	abc := textWidth("abc", 16)
	if !(ab > 0 && abc > ab) {
		t.Fatalf("want 0 < %g < %g", ab, abc)
	}
	if big := textWidth("ab", 32); big <= ab {
		t.Fatalf("size 32 (%g) should be wider than 16 (%g)", big, ab)
	}
}

func TestFontFaceConcurrent(t *testing.T) { // relevante com -race
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				textWidth("x", float64(10+j%5))
			}
		}()
	}
	wg.Wait()
}
