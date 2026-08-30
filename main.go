// noxy_game_engine — engine 2D estilo pygame para Noxy, empacotada como
// extensão por processo (kind = "process" em noxy_ext.toml).
//
// O SDK (noxyplugin) serve stdin/stdout numa goroutine; a thread principal
// espera game_init e então roda o loop do Ebiten, que exige a main.
package main

import (
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
