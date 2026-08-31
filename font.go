// font.go — fonte Go Regular embutida; uma face por tamanho, cacheada.
//
// O GoTextFaceSource do Ebiten não é seguro para uso concorrente (o shaping
// muta a face interna), e aqui há duas goroutines: o loop do Ebiten (Draw)
// e a do SDK (game_text_width). Por isso fontMu protege o *uso* da face, não
// só o cache — todo acesso passa por textWidth ou drawText.
package main

import (
	"bytes"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	fontOnce   sync.Once
	fontSource *text.GoTextFaceSource
	fontMu     sync.Mutex
	fontFaces  = map[float64]*text.GoTextFace{}
)

// fontFace devolve a face do tamanho pedido. Chamar com fontMu travado.
func fontFace(size float64) text.Face {
	fontOnce.Do(func() {
		src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		if err != nil {
			panic("goregular: " + err.Error()) // fonte embutida: nunca falha
		}
		fontSource = src
	})
	if f, ok := fontFaces[size]; ok {
		return f
	}
	f := &text.GoTextFace{Source: fontSource, Size: size}
	fontFaces[size] = f
	return f
}

// textWidth mede a largura de s na fonte embutida, em px. Headless: não
// precisa de janela nem de game_init.
func textWidth(s string, size float64) float64 {
	fontMu.Lock()
	defer fontMu.Unlock()
	w, _ := text.Measure(s, fontFace(size), 0)
	return w
}

// drawText desenha s em dst com as opções dadas (usado pelo render).
func drawText(dst *ebiten.Image, s string, size float64, op *text.DrawOptions) {
	fontMu.Lock()
	defer fontMu.Unlock()
	text.Draw(dst, s, fontFace(size), op)
}
