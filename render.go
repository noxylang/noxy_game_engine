// render.go — a ponte com o Ebiten: Update = tick, Draw = executa a lista
// atual de comandos, Layout = tamanho fixo da janela.
package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type game struct{ e *engine }

func (g *game) Update() error {
	if g.e.tick(ebitenInput{}) {
		return ebiten.Termination
	}
	return nil
}

func (g *game) Layout(int, int) (int, int) { return g.e.width, g.e.height }

func (g *game) Draw(screen *ebiten.Image) {
	for _, c := range g.e.currentFrame() {
		g.draw(screen, c)
	}
}

func (g *game) draw(screen *ebiten.Image, c command) {
	clr := color.RGBA{c.color.R, c.color.G, c.color.B, 255}
	switch c.kind {
	case cmdClear:
		screen.Fill(clr)
	case cmdRect:
		if c.thickness == 0 {
			vector.FillRect(screen, float32(c.x), float32(c.y), float32(c.w), float32(c.h), clr, false)
		} else {
			vector.StrokeRect(screen, float32(c.x), float32(c.y), float32(c.w), float32(c.h), float32(c.thickness), clr, false)
		}
	case cmdCircle:
		if c.thickness == 0 {
			vector.FillCircle(screen, float32(c.x), float32(c.y), float32(c.w), clr, true)
		} else {
			vector.StrokeCircle(screen, float32(c.x), float32(c.y), float32(c.w), float32(c.thickness), clr, true)
		}
	case cmdLine:
		vector.StrokeLine(screen, float32(c.x), float32(c.y), float32(c.w), float32(c.h), float32(c.thickness), clr, true)
	case cmdText:
		op := &text.DrawOptions{}
		op.GeoM.Translate(c.x, c.y)
		op.ColorScale.ScaleWithColor(clr)
		text.Draw(screen, c.text, fontFace(c.size), op)
	case cmdImage:
		entry, err := g.e.image(c.image) // ids validados no flip; aqui nunca falha
		if err != nil {
			return
		}
		if entry.tex == nil {
			entry.tex = ebiten.NewImageFromImage(entry.src)
		}
		w := float64(entry.tex.Bounds().Dx()) * c.scale
		h := float64(entry.tex.Bounds().Dy()) * c.scale
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(c.scale, c.scale)
		if c.angle != 0 {
			op.GeoM.Translate(-w/2, -h/2)
			op.GeoM.Rotate(c.angle * math.Pi / 180)
			op.GeoM.Translate(w/2, h/2)
		}
		op.GeoM.Translate(c.x, c.y)
		screen.DrawImage(entry.tex, op)
	}
}

// ebitenInput lê o input real; só é chamado dentro de Update.
type ebitenInput struct{}

func (ebitenInput) keysDown() []string {
	return keyNames(inpututil.AppendPressedKeys(nil))
}

func (ebitenInput) keysPressed() []string {
	return keyNames(inpututil.AppendJustPressedKeys(nil))
}

func keyNames(keys []ebiten.Key) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyName(k))
	}
	return out
}

var mouseButtons = []ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight, ebiten.MouseButtonMiddle}

func (ebitenInput) mouse() (int, int, []string, []string) {
	x, y := ebiten.CursorPosition()
	down, pressed := []string{}, []string{}
	for _, b := range mouseButtons {
		if ebiten.IsMouseButtonPressed(b) {
			down = append(down, mouseButtonName(b))
		}
		if inpututil.IsMouseButtonJustPressed(b) {
			pressed = append(pressed, mouseButtonName(b))
		}
	}
	return x, y, down, pressed
}

func (ebitenInput) windowClosing() bool { return ebiten.IsWindowBeingClosed() }
