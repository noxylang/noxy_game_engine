package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestKeyName(t *testing.T) {
	cases := map[ebiten.Key]string{
		ebiten.KeyA:            "a",
		ebiten.KeyZ:            "z",
		ebiten.KeyDigit0:       "0",
		ebiten.KeyDigit9:       "9",
		ebiten.KeyArrowLeft:    "left",
		ebiten.KeyArrowRight:   "right",
		ebiten.KeyArrowUp:      "up",
		ebiten.KeyArrowDown:    "down",
		ebiten.KeySpace:        "space",
		ebiten.KeyEnter:        "enter",
		ebiten.KeyEscape:       "escape",
		ebiten.KeyTab:          "tab",
		ebiten.KeyBackspace:    "backspace",
		ebiten.KeyShiftLeft:    "shift",
		ebiten.KeyShiftRight:   "shift",
		ebiten.KeyControlLeft:  "ctrl",
		ebiten.KeyControlRight: "ctrl",
		ebiten.KeyAltLeft:      "alt",
		ebiten.KeyAltRight:     "alt",
		ebiten.KeyF1:           "f1",
		ebiten.KeyF12:          "f12",
		ebiten.KeyNumpad5:      "numpad5",
	}
	for k, want := range cases {
		if got := keyName(k); got != want {
			t.Errorf("keyName(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestMouseButtonName(t *testing.T) {
	if mouseButtonName(ebiten.MouseButtonLeft) != "left" ||
		mouseButtonName(ebiten.MouseButtonRight) != "right" ||
		mouseButtonName(ebiten.MouseButtonMiddle) != "middle" {
		t.Fatal("mouse button names")
	}
}
