// pads.go — gamepads traduzidos para o vocabulário do wrapper. A numeração
// dos controles é a posição na lista de conectados (o primeiro é sempre 0),
// não o id bruto do Ebiten.
//
// Dois modos por controle. Com layout padrão (o Ebiten sabe qual botão é o
// "A"), os nomes são os do layout: "a", "start", "left_x". Sem ele — o caso
// de muitos controles genéricos USB —, caem os nomes brutos do dispositivo:
// "b0".."bN" e "a0".."aN". Assim todo controle é utilizável; quem quiser os
// nomes bonitos passa uma linha do SDL_GameControllerDB para
// add_gamepad_mapping.
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
	padName(pad int) string
	standard(pad int) bool

	// layout padrão
	pressedButtons(pad int) []ebiten.StandardGamepadButton
	justPressedButtons(pad int) []ebiten.StandardGamepadButton
	axisValue(pad int, a ebiten.StandardGamepadAxis) float64
	buttonValue(pad int, b ebiten.StandardGamepadButton) float64

	// bruto (controles sem layout padrão)
	rawPressedButtons(pad int) []ebiten.GamepadButton
	rawJustPressedButtons(pad int) []ebiten.GamepadButton
	rawAxisCount(pad int) int
	rawAxisValue(pad int, axis int) float64
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

// padAxisNames são os eixos do layout padrão, na ordem em que gatherPads os
// emite.
var padAxisNames = []string{"left_x", "left_y", "right_x", "right_y", "lt", "rt"}

// padAxisIndex é a posição do eixo padrão na lista, ou -1.
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

// padData é o que vai para o snapshot. Botões e eixos são nomeados com o
// número do controle na frente ("0:a", "1:b3"); axisNames e axisValues são
// paralelos, porque a quantidade de eixos varia por dispositivo.
type padData struct {
	count      int
	names      []string // nome do dispositivo, índice = número do controle
	kinds      []string // "standard" ou "raw", índice = número do controle
	down       []string
	pressed    []string
	axisNames  []string
	axisValues []float64
}

func newPadData() padData {
	return padData{
		names:      []string{},
		kinds:      []string{},
		down:       []string{},
		pressed:    []string{},
		axisNames:  []string{},
		axisValues: []float64{},
	}
}

// gatherPads lê todos os controles conectados.
func gatherPads(src padSource) padData {
	out := newPadData()
	out.count = src.padCount()
	for pad := 0; pad < out.count; pad++ {
		prefix := strconv.Itoa(pad) + ":"
		out.names = append(out.names, src.padName(pad))
		if src.standard(pad) {
			out.kinds = append(out.kinds, "standard")
			gatherStandardPad(src, pad, prefix, &out)
		} else {
			out.kinds = append(out.kinds, "raw")
			gatherRawPad(src, pad, prefix, &out)
		}
	}
	return out
}

func gatherStandardPad(src padSource, pad int, prefix string, out *padData) {
	for _, b := range src.pressedButtons(pad) {
		if name, ok := padButtonNames[b]; ok {
			out.down = append(out.down, prefix+name)
		}
	}
	for _, b := range src.justPressedButtons(pad) {
		if name, ok := padButtonNames[b]; ok {
			out.pressed = append(out.pressed, prefix+name)
		}
	}
	for i, a := range padStickAxes {
		out.axisNames = append(out.axisNames, prefix+padAxisNames[i])
		out.axisValues = append(out.axisValues, src.axisValue(pad, a))
	}
	out.axisNames = append(out.axisNames, prefix+"lt", prefix+"rt")
	out.axisValues = append(out.axisValues,
		src.buttonValue(pad, ebiten.StandardGamepadButtonFrontBottomLeft),
		src.buttonValue(pad, ebiten.StandardGamepadButtonFrontBottomRight))
}

func gatherRawPad(src padSource, pad int, prefix string, out *padData) {
	for _, b := range src.rawPressedButtons(pad) {
		out.down = append(out.down, prefix+rawButtonName(b))
	}
	for _, b := range src.rawJustPressedButtons(pad) {
		out.pressed = append(out.pressed, prefix+rawButtonName(b))
	}
	for i := 0; i < src.rawAxisCount(pad); i++ {
		out.axisNames = append(out.axisNames, prefix+"a"+strconv.Itoa(i))
		out.axisValues = append(out.axisValues, src.rawAxisValue(pad, i))
	}
}

func rawButtonName(b ebiten.GamepadButton) string {
	return "b" + strconv.Itoa(int(b))
}
