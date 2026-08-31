// noxy_game_engine — engine 2D estilo pygame para Noxy, empacotada como
// extensão por processo (kind = "process" em noxy_ext.toml).
//
// O SDK (noxyplugin) serve stdin/stdout numa goroutine; a thread principal
// espera game_init e então roda o loop do Ebiten, que exige a main.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/estevaofon/noxy/sdk/noxyplugin"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	e := newEngine()
	p := noxyplugin.New()
	p.Handle("game_init", noxyplugin.Func3(e.handleInit))
	p.Handle("game_flip", noxyplugin.Func1(e.handleFlip))
	p.Handle("game_load_image", noxyplugin.Func1(e.handleLoadImage))
	p.Handle("game_quit", noxyplugin.Func0(e.handleQuit))
	p.Handle("game_text_width", noxyplugin.Func2(handleTextWidth))

	a := newAudioEngine() // independente da janela: não exige game_init
	p.Handle("game_load_sound", noxyplugin.Func1(a.handleLoadSound))
	p.Handle("game_play_sound", noxyplugin.Func1(a.handlePlaySound))
	p.Handle("game_play_music", noxyplugin.Func1(a.handlePlayMusic))
	p.Handle("game_stop_music", noxyplugin.Func0(a.handleStopMusic))
	p.Handle("game_set_volume", noxyplugin.Func1(a.handleSetVolume))
	go p.Main() // sai do processo (os.Exit) quando o host fecha o stdin

	req := <-e.initReq
	ebiten.SetWindowSize(req.width, req.height)
	ebiten.SetWindowTitle(req.title)
	ebiten.SetWindowClosingHandled(true) // o X só marca closed; o script decide
	err := ebiten.RunGame(&game{e: e})
	if err != nil && !errors.Is(err, ebiten.Termination) {
		e.markFailed(err)
		fmt.Fprintln(os.Stderr, "[ext game]", err)
	}
	e.markQuit()
	select {} // a janela fechou; o processo vive até o host fechar o stdin
}

// handleTextWidth: game_text_width(s, size) -> float. Não exige game_init.
func handleTextWidth(ctx context.Context, s string, size int64) (float64, error) {
	if size <= 0 {
		return 0, fmt.Errorf("size must be positive, got %d", size)
	}
	return textWidth(s, float64(size), 0)
}
