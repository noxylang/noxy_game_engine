package main

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeInput struct {
	down, pressed []string
	mx, my        int
	mdown, mpress []string
	closing       bool
}

func (f fakeInput) keysDown() []string    { return f.down }
func (f fakeInput) keysPressed() []string { return f.pressed }
func (f fakeInput) mouse() (int, int, []string, []string) {
	return f.mx, f.my, f.mdown, f.mpress
}
func (f fakeInput) windowClosing() bool { return f.closing }

// startedEngine simula main(): consome o initRequest e sinaliza ready.
func startedEngine(t *testing.T) *engine {
	t.Helper()
	e := newEngine()
	done := make(chan error, 1)
	go func() {
		_, err := e.handleInit(context.Background(), 320, 240, "t")
		done <- err
	}()
	req := <-e.initReq
	if req.width != 320 || req.height != 240 || req.title != "t" {
		t.Fatalf("init request %+v", req)
	}
	e.markReady()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return e
}

// waitPending espera o frame de um flip concorrente ficar pendente.
func waitPending(t *testing.T, e *engine) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !e.hasPendingFrame() {
		if time.Now().After(deadline) {
			t.Fatal("frame never became pending")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCallsBeforeInitFail(t *testing.T) {
	e := newEngine()
	_, err := e.handleFlip(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "game not initialized: call game.init(width, height, title) first") {
		t.Fatalf("flip before init: %v", err)
	}
	if _, err := e.handleLoadImage(context.Background(), "x.png"); err == nil {
		t.Fatal("load_image before init should fail")
	}
	if _, err := e.handleQuit(context.Background()); err == nil {
		t.Fatal("quit before init should fail")
	}
}

func TestInitRejectsBadSize(t *testing.T) {
	e := newEngine()
	_, err := e.handleInit(context.Background(), 0, 10, "t")
	if err == nil || !strings.Contains(err.Error(), "window size must be positive") {
		t.Fatalf("got %v", err)
	}
}

func TestInitTwiceFails(t *testing.T) {
	e := startedEngine(t)
	_, err := e.handleInit(context.Background(), 1, 1, "again")
	if err == nil || !strings.Contains(err.Error(), "game already initialized") {
		t.Fatalf("second init: %v", err)
	}
}

func TestInitFailsWhenWindowCannotOpen(t *testing.T) {
	e := newEngine()
	done := make(chan error, 1)
	go func() {
		_, err := e.handleInit(context.Background(), 1, 1, "t")
		done <- err
	}()
	<-e.initReq
	e.markFailed(errors.New("no display"))
	err := <-done
	if err == nil || !strings.Contains(err.Error(), "could not open the window: no display") {
		t.Fatalf("got %v", err)
	}
}

func TestFlipDeliversFrameAndSnapshot(t *testing.T) {
	e := startedEngine(t)
	type result struct {
		snap map[string]any
		err  error
	}
	res := make(chan result, 1)
	go func() {
		s, err := e.handleFlip(context.Background(), []any{[]any{"clear", int64(1), int64(2), int64(3)}})
		res <- result{s, err}
	}()
	waitPending(t, e)
	in := fakeInput{down: []string{"left"}, pressed: []string{"space"}, mx: 5, my: 6, mdown: []string{"left"}, mpress: []string{"left"}}
	if e.tick(in) {
		t.Fatal("tick asked to terminate")
	}
	r := <-res
	if r.err != nil {
		t.Fatal(r.err)
	}
	if got := e.currentFrame(); len(got) != 1 || got[0].kind != cmdClear {
		t.Fatalf("current frame %+v", got)
	}
	if r.snap["closed"] != false || r.snap["mouse_x"] != int64(5) || r.snap["mouse_y"] != int64(6) {
		t.Errorf("snapshot %+v", r.snap)
	}
	if kd := r.snap["keys_down"].([]string); len(kd) != 1 || kd[0] != "left" {
		t.Errorf("keys_down %+v", r.snap["keys_down"])
	}
	if kp := r.snap["keys_pressed"].([]string); len(kp) != 1 || kp[0] != "space" {
		t.Errorf("keys_pressed %+v", r.snap["keys_pressed"])
	}
	if md := r.snap["mouse_down"].([]string); len(md) != 1 || md[0] != "left" {
		t.Errorf("mouse_down %+v", r.snap["mouse_down"])
	}
	if _, ok := r.snap["dt"].(float64); !ok {
		t.Errorf("dt %+v", r.snap["dt"])
	}
	// um tick sem frame pendente mantém o frame atual e não entrega nada
	if e.tick(fakeInput{}) || len(e.currentFrame()) != 1 {
		t.Fatal("idle tick changed state")
	}
}

func TestFlipRejectsBadFrameWithoutReplacingCurrent(t *testing.T) {
	e := startedEngine(t)
	_, err := e.handleFlip(context.Background(), []any{[]any{"nope"}})
	if err == nil || !strings.Contains(err.Error(), `command 0: unknown tag "nope"`) {
		t.Fatalf("got %v", err)
	}
	if e.hasPendingFrame() {
		t.Fatal("bad frame must not be queued")
	}
}

func TestFlipRejectsUnknownImage(t *testing.T) {
	e := startedEngine(t)
	_, err := e.handleFlip(context.Background(), []any{[]any{"image", int64(42), 0.0, 0.0, 1.0, 0.0}})
	if err == nil || !strings.Contains(err.Error(), "command 0: unknown image 42") {
		t.Fatalf("got %v", err)
	}
	if e.hasPendingFrame() {
		t.Fatal("bad frame must not be queued")
	}
}

func TestFlipHonoursContextCancel(t *testing.T) {
	e := startedEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.handleFlip(ctx, nil)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if e.hasPendingFrame() {
		t.Fatal("cancelled flip must withdraw its frame")
	}
}

func TestWindowCloseMarksClosed(t *testing.T) {
	e := startedEngine(t)
	res := make(chan map[string]any, 1)
	go func() {
		s, _ := e.handleFlip(context.Background(), nil)
		res <- s
	}()
	waitPending(t, e)
	if e.tick(fakeInput{closing: true}) {
		t.Fatal("closing the window must not terminate: the script decides")
	}
	if s := <-res; s["closed"] != true {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestQuitTerminatesAndBlocksFlip(t *testing.T) {
	e := startedEngine(t)
	if _, err := e.handleQuit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !e.tick(fakeInput{}) {
		t.Fatal("tick after quit must terminate")
	}
	if _, err := e.handleQuit(context.Background()); err != nil {
		t.Fatal("quit must be idempotent")
	}
	_, err := e.handleFlip(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "game is closed") {
		t.Fatalf("flip after quit: %v", err)
	}
}

func TestLoadImage(t *testing.T) {
	e := startedEngine(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "dot.png")
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	info, err := e.handleLoadImage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if info["id"] != int64(1) || info["width"] != int64(3) || info["height"] != int64(2) {
		t.Fatalf("info %+v", info)
	}
	entry, err := e.image(1)
	if err != nil || entry.src.Bounds().Dx() != 3 {
		t.Fatalf("image(1): %v %v", entry, err)
	}
	if _, err := e.image(99); err == nil || !strings.Contains(err.Error(), "unknown image 99") {
		t.Fatalf("image(99): %v", err)
	}
	if _, err := e.handleLoadImage(context.Background(), filepath.Join(dir, "missing.png")); err == nil {
		t.Fatal("missing file should fail")
	}
}

func TestSnapshotToMapNeverNil(t *testing.T) {
	m := inputSnapshot{}.toMap()
	for _, k := range []string{"keys_down", "keys_pressed", "mouse_down", "mouse_pressed"} {
		if v, ok := m[k].([]string); !ok || v == nil {
			t.Errorf("%s = %#v, want empty non-nil slice", k, m[k])
		}
	}
}
