// keys.go — nomes de tecla e botão no vocabulário do wrapper ("left",
// "space", "a", "0", "shift"...), derivados de ebiten.Key.String().
package main

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

func keyName(k ebiten.Key) string {
	s := k.String()
	switch {
	case strings.HasPrefix(s, "Arrow"):
		return strings.ToLower(strings.TrimPrefix(s, "Arrow"))
	case strings.HasPrefix(s, "Digit"):
		return strings.TrimPrefix(s, "Digit")
	case strings.HasPrefix(s, "Shift"):
		return "shift"
	case strings.HasPrefix(s, "Control"):
		return "ctrl"
	case strings.HasPrefix(s, "Alt"):
		return "alt"
	case strings.HasPrefix(s, "Meta"):
		return "meta"
	}
	return strings.ToLower(s)
}

func mouseButtonName(b ebiten.MouseButton) string {
	switch b {
	case ebiten.MouseButtonLeft:
		return "left"
	case ebiten.MouseButtonRight:
		return "right"
	case ebiten.MouseButtonMiddle:
		return "middle"
	}
	return ""
}
