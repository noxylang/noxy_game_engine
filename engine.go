// engine.go — estado compartilhado entre a goroutine do SDK (handlers) e o
// loop do Ebiten (tick/Draw na thread principal). Tudo aqui roda sem janela:
// o Ebiten entra só por inputReader (render.go) e pelo campo tex de
// imageEntry, preenchido pelo render.
package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decoders registrados para image.Decode
	_ "image/png"
	"os"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	errNotInitialized = errors.New("game not initialized: call game.init(width, height, title) first")
	errClosed         = errors.New("game is closed")
)

// inputReader é o que o tick lê a cada Update; render.go dá a implementação
// sobre o Ebiten, os testes dão uma fake.
type inputReader interface {
	keysDown() []string
	keysPressed() []string
	mouse() (x, y int, down, pressed []string)
	wheel() (x, y float64)
	textInput() string
	pads() (down, pressed []string, axes []float64)
	windowClosing() bool
}

// platform é o que o tick usa a cada Update: o input do frame e as
// operações de janela, que precisam rodar na thread principal. render.go dá
// a implementação sobre o Ebiten, os testes dão uma fake.
type platform interface {
	inputReader
	setFullscreen(on bool)
	setTPS(n int)
}

// inputSnapshot é o retorno de game_flip.
type inputSnapshot struct {
	Closed                  bool
	DT                      float64
	KeysDown, KeysPressed   []string
	MouseX, MouseY          int
	MouseDown, MousePressed []string
	WheelX, WheelY          float64
	TextInput               string
	PadDown, PadPressed     []string
	PadAxes                 []float64
}

func (s inputSnapshot) toMap() map[string]any {
	return map[string]any{
		"closed":        s.Closed,
		"dt":            s.DT,
		"keys_down":     nonNil(s.KeysDown),
		"keys_pressed":  nonNil(s.KeysPressed),
		"mouse_x":       int64(s.MouseX),
		"mouse_y":       int64(s.MouseY),
		"mouse_down":    nonNil(s.MouseDown),
		"mouse_pressed": nonNil(s.MousePressed),
		"wheel_x":       s.WheelX,
		"wheel_y":       s.WheelY,
		"text_input":    s.TextInput,
		"pad_down":      nonNil(s.PadDown),
		"pad_pressed":   nonNil(s.PadPressed),
		"pad_axes":      nonNilF(s.PadAxes),
	}
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

func nonNilF(xs []float64) []float64 {
	if xs == nil {
		return []float64{}
	}
	return xs
}

type initRequest struct {
	width, height int
	title         string
}

// pendingFrame é um frame entregue por game_flip à espera do próximo tick;
// reply recebe o snapshot colhido nesse tick.
type pendingFrame struct {
	cmds  []command
	reply chan inputSnapshot
}

type imageEntry struct {
	src image.Image
	tex *ebiten.Image // criado no primeiro desenho (render.go)
}

type engine struct {
	mu      sync.Mutex
	started bool // game_init aceito
	ready   bool // primeiro tick rodou: a janela existe
	quit    bool // game_quit chamado (ou RunGame retornou)
	closed  bool // usuário fechou a janela

	width, height int
	initReq       chan initRequest // main() espera aqui antes de RunGame
	readyCh       chan struct{}    // fechado por markReady
	failedCh      chan struct{}    // fechado por markFailed
	failure       error

	pending  *pendingFrame
	current  []command
	lastTick time.Time

	pendingFullscreen *bool // pedidos de janela, aplicados pelo tick
	pendingTPS        *int

	images    map[int64]*imageEntry
	nextImage int64
}

func newEngine() *engine {
	return &engine{
		initReq:  make(chan initRequest, 1),
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
		images:   map[int64]*imageEntry{},
	}
}

// handleInit: game_init(width, height, title) -> void.
func (e *engine) handleInit(ctx context.Context, width, height int64, title string) (any, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("window size must be positive, got %dx%d", width, height)
	}
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return nil, errors.New("game already initialized")
	}
	e.started = true
	e.width, e.height = int(width), int(height)
	e.mu.Unlock()
	e.initReq <- initRequest{width: int(width), height: int(height), title: title}
	select {
	case <-e.readyCh:
		return nil, nil
	case <-e.failedCh:
		return nil, fmt.Errorf("could not open the window: %w", e.failure)
	case <-ctx.Done():
		return nil, fmt.Errorf("window did not open in time: %w", ctx.Err())
	}
}

// markReady é chamado pelo primeiro tick; markFailed por main() se RunGame
// falhar antes disso.
func (e *engine) markReady() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.ready {
		e.ready = true
		e.lastTick = time.Now()
		close(e.readyCh)
	}
}

func (e *engine) markFailed(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.failure == nil {
		e.failure = err
		close(e.failedCh)
	}
}

// handleFlip: game_flip(cmds) -> snapshot. Valida o frame inteiro (tags e
// ids de imagem), publica-o e espera o tick que o consumir.
func (e *engine) handleFlip(ctx context.Context, raw []any) (map[string]any, error) {
	cmds, err := decodeFrame(raw)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	if !e.started {
		e.mu.Unlock()
		return nil, errNotInitialized
	}
	if e.quit {
		e.mu.Unlock()
		return nil, errClosed
	}
	for i, c := range cmds {
		if c.kind == cmdText {
			// ordem de locks: sempre e.mu -> fontMu
			if !fontExists(c.font) {
				e.mu.Unlock()
				return nil, fmt.Errorf("command %d: unknown font %d (not returned by load_font)", i, c.font)
			}
			continue
		}
		if c.kind != cmdImage {
			continue
		}
		entry, ok := e.images[c.image]
		if !ok {
			e.mu.Unlock()
			return nil, fmt.Errorf("command %d: unknown image %d (not returned by load_image)", i, c.image)
		}
		b := entry.src.Bounds()
		if c.srcX < 0 || c.srcY < 0 || c.srcX+c.srcW > float64(b.Dx()) || c.srcY+c.srcH > float64(b.Dy()) {
			e.mu.Unlock()
			return nil, fmt.Errorf(`command %d: element 2 of "image": source rect %g,%g %gx%g outside image %d (%dx%d)`,
				i, c.srcX, c.srcY, c.srcW, c.srcH, c.image, b.Dx(), b.Dy())
		}
	}
	frame := &pendingFrame{cmds: cmds, reply: make(chan inputSnapshot, 1)}
	e.pending = frame
	e.mu.Unlock()
	select {
	case snap := <-frame.reply:
		return snap.toMap(), nil
	case <-ctx.Done():
		e.mu.Lock()
		if e.pending == frame {
			e.pending = nil
		}
		e.mu.Unlock()
		return nil, fmt.Errorf("frame was not displayed: %w", ctx.Err())
	}
}

func (e *engine) hasPendingFrame() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pending != nil
}

// tick é o corpo de Update: consome o frame pendente (se houver), colhe o
// input e responde ao flip que o entregou. Devolve true quando o Ebiten
// deve encerrar (game_quit).
func (e *engine) tick(p platform) (terminate bool) {
	e.markReady()
	e.mu.Lock()
	if e.quit {
		e.mu.Unlock()
		return true
	}
	if p.windowClosing() {
		e.closed = true
	}
	fs, tps := e.pendingFullscreen, e.pendingTPS
	e.pendingFullscreen, e.pendingTPS = nil, nil
	frame := e.pending
	e.pending = nil
	if frame != nil {
		e.current = frame.cmds
	}
	now := time.Now()
	dt := now.Sub(e.lastTick).Seconds()
	if frame != nil {
		e.lastTick = now
	}
	closed := e.closed
	e.mu.Unlock()
	if fs != nil {
		p.setFullscreen(*fs)
	}
	if tps != nil {
		p.setTPS(*tps)
	}
	if frame == nil {
		return false
	}
	mx, my, mdown, mpressed := p.mouse()
	wx, wy := p.wheel()
	pdown, ppressed, paxes := p.pads()
	frame.reply <- inputSnapshot{
		Closed:       closed,
		DT:           dt,
		KeysDown:     p.keysDown(),
		KeysPressed:  p.keysPressed(),
		MouseX:       mx,
		MouseY:       my,
		MouseDown:    mdown,
		MousePressed: mpressed,
		WheelX:       wx,
		WheelY:       wy,
		TextInput:    p.textInput(),
		PadDown:      pdown,
		PadPressed:   ppressed,
		PadAxes:      paxes,
	}
	return false
}

// currentFrame é a lista que Draw redesenha até chegar outra.
func (e *engine) currentFrame() []command {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.current
}

// handleQuit: game_quit() -> void. Idempotente; o tick seguinte encerra.
func (e *engine) handleQuit(ctx context.Context) (any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil, errNotInitialized
	}
	e.quit = true
	e.closed = true
	return nil, nil
}

// handleSetFullscreen: game_set_fullscreen(on) -> void. Aplicado pelo tick
// seguinte, na thread principal.
func (e *engine) handleSetFullscreen(ctx context.Context, on bool) (any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil, errNotInitialized
	}
	e.pendingFullscreen = &on
	return nil, nil
}

// handleSetFps: game_set_fps(n) -> void.
func (e *engine) handleSetFps(ctx context.Context, n int64) (any, error) {
	if n < 1 {
		return nil, fmt.Errorf("fps must be at least 1, got %d", n)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil, errNotInitialized
	}
	tps := int(n)
	e.pendingTPS = &tps
	return nil, nil
}

// markQuit é chamado por main() quando RunGame retorna por qualquer motivo.
func (e *engine) markQuit() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.quit = true
	e.closed = true
}

// handleLoadImage: game_load_image(path) -> {"id", "width", "height"}.
// Só decodifica aqui; a textura do Ebiten nasce no primeiro desenho.
func (e *engine) handleLoadImage(ctx context.Context, path string) (map[string]any, error) {
	e.mu.Lock()
	started := e.started
	e.mu.Unlock()
	if !started {
		return nil, errNotInitialized
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextImage++
	e.images[e.nextImage] = &imageEntry{src: src}
	b := src.Bounds()
	return map[string]any{"id": e.nextImage, "width": int64(b.Dx()), "height": int64(b.Dy())}, nil
}

func (e *engine) image(id int64) (*imageEntry, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, ok := e.images[id]
	if !ok {
		return nil, fmt.Errorf("unknown image %d (not returned by load_image)", id)
	}
	return entry, nil
}
