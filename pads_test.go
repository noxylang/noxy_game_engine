// pads_test.go — a montagem dos nomes de gamepad, com uma fonte fake: não
// há controle no CI, então o que se testa é a tabela e o formato.
package main

import (
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// fakePads implementa padSource com dados fixos.
type fakePads struct {
	count   int
	down    map[int][]ebiten.StandardGamepadButton
	just    map[int][]ebiten.StandardGamepadButton
	axes    map[int]map[ebiten.StandardGamepadAxis]float64
	trigger map[int]map[ebiten.StandardGamepadButton]float64
}

func (f fakePads) padCount() int { return f.count }

func (f fakePads) pressedButtons(pad int) []ebiten.StandardGamepadButton {
	return f.down[pad]
}

func (f fakePads) justPressedButtons(pad int) []ebiten.StandardGamepadButton {
	return f.just[pad]
}

func (f fakePads) axisValue(pad int, a ebiten.StandardGamepadAxis) float64 {
	return f.axes[pad][a]
}

func (f fakePads) buttonValue(pad int, b ebiten.StandardGamepadButton) float64 {
	return f.trigger[pad][b]
}

func TestGatherPadsEmpty(t *testing.T) {
	down, pressed, axes := gatherPads(fakePads{})
	if len(down) != 0 || len(pressed) != 0 || len(axes) != 0 {
		t.Fatalf("sem controles: %v %v %v", down, pressed, axes)
	}
}

func TestGatherPadsNames(t *testing.T) {
	src := fakePads{
		count: 2,
		down: map[int][]ebiten.StandardGamepadButton{
			0: {ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonLeftTop},
			1: {ebiten.StandardGamepadButtonCenterRight},
		},
		just: map[int][]ebiten.StandardGamepadButton{
			1: {ebiten.StandardGamepadButtonFrontTopLeft},
		},
	}
	down, pressed, axes := gatherPads(src)
	wantDown := []string{"0:a", "0:up", "1:start"}
	if !reflect.DeepEqual(down, wantDown) {
		t.Fatalf("down: want %v, got %v", wantDown, down)
	}
	wantPressed := []string{"1:lb"}
	if !reflect.DeepEqual(pressed, wantPressed) {
		t.Fatalf("pressed: want %v, got %v", wantPressed, pressed)
	}
	if len(axes) != 12 { // 6 por controle
		t.Fatalf("axes: want 12 values, got %d", len(axes))
	}
}

func TestGatherPadsAxes(t *testing.T) {
	src := fakePads{
		count: 1,
		axes: map[int]map[ebiten.StandardGamepadAxis]float64{
			0: {
				ebiten.StandardGamepadAxisLeftStickHorizontal: -0.5,
				ebiten.StandardGamepadAxisRightStickVertical:  0.25,
			},
		},
		trigger: map[int]map[ebiten.StandardGamepadButton]float64{
			0: {ebiten.StandardGamepadButtonFrontBottomRight: 0.75},
		},
	}
	_, _, axes := gatherPads(src)
	want := []float64{-0.5, 0, 0, 0.25, 0, 0.75} // left_x left_y right_x right_y lt rt
	if !reflect.DeepEqual(axes, want) {
		t.Fatalf("want %v, got %v", want, axes)
	}
}

func TestPadAxisIndex(t *testing.T) {
	for i, name := range padAxisNames {
		if got := padAxisIndex(name); got != i {
			t.Fatalf("%s: want %d, got %d", name, i, got)
		}
	}
	if padAxisIndex("nope") != -1 {
		t.Fatal("eixo desconhecido deve dar -1")
	}
}
