// pads_test.go — a montagem dos nomes de gamepad, com uma fonte fake: não
// há controle no CI, então o que se testa é a tabela e o formato, nos dois
// modos (layout padrão e bruto).
package main

import (
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// fakePads implementa padSource com dados fixos.
type fakePads struct {
	count   int
	names   map[int]string
	std     map[int]bool
	down    map[int][]ebiten.StandardGamepadButton
	just    map[int][]ebiten.StandardGamepadButton
	axes    map[int]map[ebiten.StandardGamepadAxis]float64
	trigger map[int]map[ebiten.StandardGamepadButton]float64
	rawDown map[int][]ebiten.GamepadButton
	rawJust map[int][]ebiten.GamepadButton
	rawAxes map[int][]float64
}

func (f fakePads) padCount() int          { return f.count }
func (f fakePads) padName(pad int) string { return f.names[pad] }
func (f fakePads) standard(pad int) bool  { return f.std[pad] }

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

func (f fakePads) rawPressedButtons(pad int) []ebiten.GamepadButton {
	return f.rawDown[pad]
}

func (f fakePads) rawJustPressedButtons(pad int) []ebiten.GamepadButton {
	return f.rawJust[pad]
}

func (f fakePads) rawAxisCount(pad int) int { return len(f.rawAxes[pad]) }

func (f fakePads) rawAxisValue(pad int, axis int) float64 {
	return f.rawAxes[pad][axis]
}

func TestGatherPadsEmpty(t *testing.T) {
	d := gatherPads(fakePads{})
	if d.count != 0 || len(d.down) != 0 || len(d.axisNames) != 0 || len(d.axisValues) != 0 {
		t.Fatalf("sem controles: %+v", d)
	}
}

func TestGatherPadsStandard(t *testing.T) {
	src := fakePads{
		count: 2,
		names: map[int]string{0: "Xbox", 1: "Xbox"},
		std:   map[int]bool{0: true, 1: true},
		down: map[int][]ebiten.StandardGamepadButton{
			0: {ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonLeftTop},
			1: {ebiten.StandardGamepadButtonCenterRight},
		},
		just: map[int][]ebiten.StandardGamepadButton{
			1: {ebiten.StandardGamepadButtonFrontTopLeft},
		},
	}
	d := gatherPads(src)
	if want := []string{"0:a", "0:up", "1:start"}; !reflect.DeepEqual(d.down, want) {
		t.Fatalf("down: want %v, got %v", want, d.down)
	}
	if want := []string{"1:lb"}; !reflect.DeepEqual(d.pressed, want) {
		t.Fatalf("pressed: want %v, got %v", want, d.pressed)
	}
	if want := []string{"standard", "standard"}; !reflect.DeepEqual(d.kinds, want) {
		t.Fatalf("kinds: %v", d.kinds)
	}
	if len(d.axisNames) != 12 || len(d.axisValues) != 12 { // 6 por controle
		t.Fatalf("axes: %d nomes, %d valores", len(d.axisNames), len(d.axisValues))
	}
	if d.axisNames[0] != "0:left_x" || d.axisNames[5] != "0:rt" || d.axisNames[6] != "1:left_x" {
		t.Fatalf("axis names: %v", d.axisNames)
	}
}

func TestGatherPadsStandardAxisValues(t *testing.T) {
	src := fakePads{
		count: 1,
		names: map[int]string{0: "Xbox"},
		std:   map[int]bool{0: true},
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
	d := gatherPads(src)
	want := []float64{-0.5, 0, 0, 0.25, 0, 0.75} // left_x left_y right_x right_y lt rt
	if !reflect.DeepEqual(d.axisValues, want) {
		t.Fatalf("want %v, got %v", want, d.axisValues)
	}
}

// Um controle genérico USB (o caso que motivou o fallback): sem layout
// padrão, os nomes viram os índices brutos do dispositivo.
func TestGatherPadsRawFallback(t *testing.T) {
	src := fakePads{
		count:   1,
		names:   map[int]string{0: "Generic   USB  Joystick  "},
		std:     map[int]bool{0: false},
		rawDown: map[int][]ebiten.GamepadButton{0: {ebiten.GamepadButton2, ebiten.GamepadButton14}},
		rawJust: map[int][]ebiten.GamepadButton{0: {ebiten.GamepadButton2}},
		rawAxes: map[int][]float64{0: {0.0, -1.0, 0.5, 0.0, 0.25}},
	}
	d := gatherPads(src)
	if d.count != 1 || d.kinds[0] != "raw" || d.names[0] != "Generic   USB  Joystick  " {
		t.Fatalf("pad: %+v", d)
	}
	if want := []string{"0:b2", "0:b14"}; !reflect.DeepEqual(d.down, want) {
		t.Fatalf("down: want %v, got %v", want, d.down)
	}
	if want := []string{"0:b2"}; !reflect.DeepEqual(d.pressed, want) {
		t.Fatalf("pressed: want %v, got %v", want, d.pressed)
	}
	wantNames := []string{"0:a0", "0:a1", "0:a2", "0:a3", "0:a4"}
	if !reflect.DeepEqual(d.axisNames, wantNames) {
		t.Fatalf("axis names: want %v, got %v", wantNames, d.axisNames)
	}
	wantValues := []float64{0, -1, 0.5, 0, 0.25}
	if !reflect.DeepEqual(d.axisValues, wantValues) {
		t.Fatalf("axis values: want %v, got %v", wantValues, d.axisValues)
	}
}

// Padrão e bruto convivem: cada controle é numerado pela posição.
func TestGatherPadsMixed(t *testing.T) {
	src := fakePads{
		count:   2,
		names:   map[int]string{0: "Xbox", 1: "Generic"},
		std:     map[int]bool{0: true, 1: false},
		down:    map[int][]ebiten.StandardGamepadButton{0: {ebiten.StandardGamepadButtonRightBottom}},
		rawDown: map[int][]ebiten.GamepadButton{1: {ebiten.GamepadButton3}},
		rawAxes: map[int][]float64{1: {0.5}},
	}
	d := gatherPads(src)
	if want := []string{"0:a", "1:b3"}; !reflect.DeepEqual(d.down, want) {
		t.Fatalf("down: want %v, got %v", want, d.down)
	}
	if want := []string{"standard", "raw"}; !reflect.DeepEqual(d.kinds, want) {
		t.Fatalf("kinds: %v", d.kinds)
	}
	if len(d.axisNames) != 7 { // 6 do padrão + 1 bruto
		t.Fatalf("axis names: %v", d.axisNames)
	}
	if d.axisNames[6] != "1:a0" || d.axisValues[6] != 0.5 {
		t.Fatalf("raw axis: %v %v", d.axisNames[6], d.axisValues[6])
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
