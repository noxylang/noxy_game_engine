// font.go — fontes de texto: a Go Regular embutida (id 0) e as carregadas
// por load_font (id 1, 2, ...). Uma face por par (fonte, tamanho).
//
// O GoTextFaceSource do Ebiten não é seguro para uso concorrente (o shaping
// muta a face interna), e aqui há duas goroutines: o loop do Ebiten (Draw)
// e a do SDK (game_text_width / game_load_font). Por isso fontMu protege o
// registro, o cache e o *uso* das faces — todo acesso passa por textWidth
// ou drawText.
package main

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// faceKey identifica uma face: qual fonte, em que tamanho.
type faceKey struct {
	font int64
	size float64
}

var (
	fontMu      sync.Mutex
	builtinOnce sync.Once
	builtinFont *text.GoTextFaceSource
	loadedFonts = map[int64]*text.GoTextFaceSource{}
	nextFontID  int64
	fontFaces   = map[faceKey]*text.GoTextFace{}
)

// builtinSource devolve a Go Regular embutida. Chamar com fontMu travado.
func builtinSource() *text.GoTextFaceSource {
	builtinOnce.Do(func() {
		src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		if err != nil {
			panic("goregular: " + err.Error()) // fonte embutida: nunca falha
		}
		builtinFont = src
	})
	return builtinFont
}

// source resolve um id de fonte. Chamar com fontMu travado.
func source(id int64) (*text.GoTextFaceSource, error) {
	if id == 0 {
		return builtinSource(), nil
	}
	src, ok := loadedFonts[id]
	if !ok {
		return nil, fmt.Errorf("unknown font %d (not returned by load_font)", id)
	}
	return src, nil
}

// face devolve a face do par (fonte, tamanho). Chamar com fontMu travado.
func face(id int64, size float64) (text.Face, error) {
	k := faceKey{font: id, size: size}
	if f, ok := fontFaces[k]; ok {
		return f, nil
	}
	src, err := source(id)
	if err != nil {
		return nil, err
	}
	f := &text.GoTextFace{Source: src, Size: size}
	fontFaces[k] = f
	return f, nil
}

// loadFont registra um TTF/OTF e devolve o id (1, 2, ...).
func loadFont(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	fontMu.Lock()
	defer fontMu.Unlock()
	nextFontID++
	loadedFonts[nextFontID] = src
	return nextFontID, nil
}

// fontExists diz se o id é usável num frame (0 é sempre a embutida).
func fontExists(id int64) bool {
	if id == 0 {
		return true
	}
	fontMu.Lock()
	defer fontMu.Unlock()
	_, ok := loadedFonts[id]
	return ok
}

// textWidth mede a largura de s em px. Headless: não precisa de janela nem
// de game_init.
func textWidth(s string, size float64, fontID int64) (float64, error) {
	fontMu.Lock()
	defer fontMu.Unlock()
	f, err := face(fontID, size)
	if err != nil {
		return 0, err
	}
	w, _ := text.Measure(s, f, 0)
	return w, nil
}

// drawText desenha s em dst (usado pelo render).
func drawText(dst *ebiten.Image, s string, size float64, fontID int64, op *text.DrawOptions) error {
	fontMu.Lock()
	defer fontMu.Unlock()
	f, err := face(fontID, size)
	if err != nil {
		return err
	}
	text.Draw(dst, s, f, op)
	return nil
}
