// font.go — fonte Go Regular embutida; uma face por tamanho, cacheada.
package main

import (
	"bytes"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	fontOnce   sync.Once
	fontSource *text.GoTextFaceSource
	fontMu     sync.Mutex
	fontFaces  = map[float64]*text.GoTextFace{}
)

func fontFace(size float64) text.Face {
	fontOnce.Do(func() {
		src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		if err != nil {
			panic("goregular: " + err.Error()) // fonte embutida: nunca falha
		}
		fontSource = src
	})
	fontMu.Lock()
	defer fontMu.Unlock()
	if f, ok := fontFaces[size]; ok {
		return f
	}
	f := &text.GoTextFace{Source: fontSource, Size: size}
	fontFaces[size] = f
	return f
}
