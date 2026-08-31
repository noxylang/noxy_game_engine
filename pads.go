// pads.go — gamepads no layout padrão do Ebiten, traduzidos para o
// vocabulário do wrapper. A numeração dos controles é a posição na lista de
// conectados (o primeiro é sempre 0), não o id bruto do Ebiten.
//
// A leitura real fica atrás de padSource para o teste rodar sem hardware.
package main

import (
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

// padSource é de onde gatherPads lê; render.go dá a implementação sobre o
// Ebiten, os testes dão uma fake.
type padSource interface {
	padCount() int
	pressedButtons(pad int) []ebiten.StandardGamepadButton
	justPressedButtons(pad int) []ebiten.StandardGamepadButton
	axisValue(pad int, a ebiten.StandardGamepadAxis) float64
	buttonValue(pad int, b ebiten.StandardGamepadButton) float64
}

// padButtonNames traduz o layout padrão (web gamepad) para os nomes do
// wrapper.
var padButtonNames = map[ebiten.StandardGamepadButton]string{
	ebiten.StandardGamepadButtonRightBottom:      "a",
	ebiten.StandardGamepadButtonRightRight:       "b",
	ebiten.StandardGamepadButtonRightLeft:        "x",
	ebiten.StandardGamepadButtonRightTop:         "y",
	ebiten.StandardGamepadButtonLeftTop:          "up",
	ebiten.StandardGamepadButtonLeftBottom:       "down",
	ebiten.StandardGamepadButtonLeftLeft:         "left",
	ebiten.StandardGamepadButtonLeftRight:        "right",
	ebiten.StandardGamepadButtonCenterLeft:       "back",
	ebiten.StandardGamepadButtonCenterRight:      "start",
	ebiten.StandardGamepadButtonCenterCenter:     "guide",
	ebiten.StandardGamepadButtonFrontTopLeft:     "lb",
	ebiten.StandardGamepadButtonFrontTopRight:    "rb",
	ebiten.StandardGamepadButtonFrontBottomLeft:  "lt",
	ebiten.StandardGamepadButtonFrontBottomRight: "rt",
	ebiten.StandardGamepadButtonLeftStick:        "lstick",
	ebiten.StandardGamepadButtonRightStick:       "rstick",
}

// padAxisNames é a ordem dos eixos em pad_axes (6 por controle).
var padAxisNames = []string{"left_x", "left_y", "right_x", "right_y", "lt", "rt"}

// padAxisIndex é a posição do eixo dentro do bloco de um controle, ou -1.
func padAxisIndex(name string) int {
	for i, n := range padAxisNames {
		if n == name {
			return i
		}
	}
	return -1
}

var padStickAxes = []ebiten.StandardGamepadAxis{
	ebiten.StandardGamepadAxisLeftStickHorizontal,
	ebiten.StandardGamepadAxisLeftStickVertical,
	ebiten.StandardGamepadAxisRightStickHorizontal,
	ebiten.StandardGamepadAxisRightStickVertical,
}

// gatherPads monta os arrays do snapshot: nomes prefixados pelo número do
// controle ("0:a") e os 6 eixos de cada um, em sequência.
func gatherPads(src padSource) (down, pressed []string, axes []float64) {
	n := src.padCount()
	down, pressed, axes = []string{}, []string{}, []float64{}
	for pad := 0; pad < n; pad++ {
		prefix := strconv.Itoa(pad) + ":"
		for _, b := range src.pressedButtons(pad) {
			if name, ok := padButtonNames[b]; ok {
				down = append(down, prefix+name)
			}
		}
		for _, b := range src.justPressedButtons(pad) {
			if name, ok := padButtonNames[b]; ok {
				pressed = append(pressed, prefix+name)
			}
		}
		for _, a := range padStickAxes {
			axes = append(axes, src.axisValue(pad, a))
		}
		axes = append(axes,
			src.buttonValue(pad, ebiten.StandardGamepadButtonFrontBottomLeft),
			src.buttonValue(pad, ebiten.StandardGamepadButtonFrontBottomRight))
	}
	return down, pressed, axes
}
