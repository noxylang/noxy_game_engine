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

	"github.com/noxylang/noxy/sdk/noxyplugin"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	e := newEngine()
	p := noxyplugin.New()
	p.Handle("game_init", noxyplugin.Func3(e.handleInit))
	p.Handle("game_flip", noxyplugin.Func1(e.handleFlip))
	p.Handle("game_load_image", noxyplugin.Func1(e.handleLoadImage))
	p.Handle("game_quit", noxyplugin.Func0(e.handleQuit))
	p.Handle("game_text_width", noxyplugin.Func3(handleTextWidth))
	p.Handle("game_load_font", noxyplugin.Func1(handleLoadFont))
	p.Handle("game_set_fullscreen", noxyplugin.Func1(e.handleSetFullscreen))
	p.Handle("game_set_fps", noxyplugin.Func1(e.handleSetFps))
	p.Handle("game_add_gamepad_mapping", noxyplugin.Func1(handleAddGamepadMapping))

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

// handleTextWidth: game_text_width(s, size, font_id) -> float. Não exige
// game_init.
func handleTextWidth(ctx context.Context, s string, size, fontID int64) (float64, error) {
	if size <= 0 {
		return 0, fmt.Errorf("size must be positive, got %d", size)
	}
	return textWidth(s, float64(size), fontID)
}

// handleAddGamepadMapping: game_add_gamepad_mapping(linha) -> void. Uma ou
// mais linhas no formato do SDL_GameControllerDB, para controles que o
// Ebiten ainda não conhece. Não exige game_init.
func handleAddGamepadMapping(ctx context.Context, mappings string) (any, error) {
	ok, err := ebiten.UpdateStandardGamepadLayoutMappings(mappings)
	if err != nil {
		return nil, fmt.Errorf("gamepad mapping: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("gamepad mapping was not applied (check the SDL_GameControllerDB line)")
	}
	return nil, nil
}

// handleLoadFont: game_load_font(path) -> {"id"}. Aceita TTF e OTF.
func handleLoadFont(ctx context.Context, path string) (map[string]any, error) {
	id, err := loadFont(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}
