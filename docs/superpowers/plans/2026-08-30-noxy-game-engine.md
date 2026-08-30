# noxy_game_engine (MVP) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Uma engine 2D estilo pygame para Noxy, publicada como extensão por processo (Go + Ebiten), com wrapper tipado, exemplos e release por plataforma.

**Architecture:** O wrapper `.nx` acumula comandos de desenho em estado de módulo e envia **um** `game_flip(cmds)` por frame; o plugin Go decodifica o frame (headless, testável), entrega-o ao loop do Ebiten na thread principal e devolve o snapshot de input colhido no `Update` seguinte. O SDK (`noxyplugin.Main`) roda numa goroutine; `ebiten.RunGame` fica na main.

**Tech Stack:** Go 1.25+, `github.com/hajimehoshi/ebiten/v2` v2.9.10 (`vector`, `text/v2`, `inpututil`), `github.com/estevaofon/noxy/sdk/noxyplugin` v0.1.0, `golang.org/x/image` (fonte Go Regular), Noxy ≥ 0.23.0.

**Spec:** `docs/superpowers/specs/2026-08-30-noxy-game-engine-design.md`

## Global Constraints

- Extensão: `name = "game"`, `kind = "process"`, `concurrency = "single"`, `min_noxy = "0.23.0"`.
- Exports: `game_init(int,int,string)->void`, `game_flip(any[])->map[string]any` (`timeout_ms = 10000`), `game_load_image(string)->map[string]any` (`stateful = true`), `game_quit()->void`.
- Erros: antes de init → `game not initialized: call game.init(width, height, title) first`; flip após quit → `game is closed`; frame inválido → `command N: ...`.
- Binários: `noxy-plugin-game-{windows-amd64.exe,linux-amd64,darwin-amd64,darwin-arm64}`; Linux/macOS exigem cgo → builds nativos no CI.
- Nada devolve `null` em silêncio; falha é erro de runtime capturável com `call_result`.
- Verificação Go: `go build ./... && go vet ./... && go test ./...`. Commits em português: `tipo(escopo): descrição`.

---

### Task 1: Decodificador de comandos (headless)

**Files:**
- Create: `commands.go`, `commands_test.go`

**Interfaces:**
- Produces: `type command struct{kind cmdKind; color color3; x, y, w, h, thickness, size, scale, angle float64; text string; image int64}`, `func decodeFrame(raw []any) ([]command, error)`, `type color3 struct{R, G, B uint8}`, constantes `cmdClear cmdRect cmdCircle cmdLine cmdText cmdImage`.

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"strings"
	"testing"
)

func TestDecodeFrameAllTags(t *testing.T) {
	raw := []any{
		[]any{"clear", int64(1), int64(2), int64(3)},
		[]any{"rect", 1.5, int64(2), 3.0, 4.0, int64(255), int64(0), int64(0), int64(0)},
		[]any{"circle", 10.0, 20.0, 5.0, int64(0), int64(255), int64(0), 2.0},
		[]any{"line", 0.0, 0.0, 9.0, 9.0, int64(0), int64(0), int64(255), 1.0},
		[]any{"text", "hi", 3.0, 4.0, int64(16), int64(9), int64(9), int64(9)},
		[]any{"image", int64(7), 1.0, 2.0, 2.0, 90.0},
	}
	cmds, err := decodeFrame(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 6 {
		t.Fatalf("got %d commands", len(cmds))
	}
	if cmds[0].kind != cmdClear || cmds[0].color != (color3{1, 2, 3}) {
		t.Errorf("clear: %+v", cmds[0])
	}
	r := cmds[1]
	if r.kind != cmdRect || r.x != 1.5 || r.y != 2 || r.w != 3 || r.h != 4 || r.color != (color3{255, 0, 0}) || r.thickness != 0 {
		t.Errorf("rect: %+v", r)
	}
	c := cmds[2]
	if c.kind != cmdCircle || c.x != 10 || c.y != 20 || c.w != 5 || c.thickness != 2 {
		t.Errorf("circle: %+v", c)
	}
	l := cmds[3]
	if l.kind != cmdLine || l.w != 9 || l.h != 9 || l.thickness != 1 || l.color != (color3{0, 0, 255}) {
		t.Errorf("line: %+v", l)
	}
	tx := cmds[4]
	if tx.kind != cmdText || tx.text != "hi" || tx.x != 3 || tx.size != 16 || tx.color != (color3{9, 9, 9}) {
		t.Errorf("text: %+v", tx)
	}
	im := cmds[5]
	if im.kind != cmdImage || im.image != 7 || im.x != 1 || im.y != 2 || im.scale != 2 || im.angle != 90 {
		t.Errorf("image: %+v", im)
	}
}

func TestDecodeFrameEmpty(t *testing.T) {
	cmds, err := decodeFrame(nil)
	if err != nil || len(cmds) != 0 {
		t.Fatalf("got %v, %v", cmds, err)
	}
}

func TestDecodeFrameErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  []any
		want string
	}{
		{"not an array", []any{"rect"}, `command 0: expected an array, got string`},
		{"empty command", []any{[]any{}}, `command 0: empty command`},
		{"tag not string", []any{[]any{int64(1)}}, `command 0: tag must be a string, got int`},
		{"unknown tag", []any{[]any{"blob"}}, `command 0: unknown tag "blob"`},
		{"arity", []any{[]any{"clear", int64(1)}}, `command 0: "clear" expects 4 elements, got 2`},
		{"number type", []any{[]any{"clear", int64(1), int64(2), int64(3)}, []any{"rect", "x", 1.0, 1.0, 1.0, int64(0), int64(0), int64(0), int64(0)}}, `command 1: element 1 of "rect": expected number, got string`},
		{"color range", []any{[]any{"clear", int64(300), int64(0), int64(0)}}, `command 0: element 1 of "clear": color component out of range 0..255, got 300`},
		{"color float", []any{[]any{"clear", 1.5, int64(0), int64(0)}}, `command 0: element 1 of "clear": color component must be an int, got float`},
		{"text type", []any{[]any{"text", int64(1), 0.0, 0.0, int64(12), int64(0), int64(0), int64(0)}}, `command 0: element 1 of "text": expected string, got int`},
		{"image id type", []any{[]any{"image", "a", 0.0, 0.0, 1.0, 0.0}}, `command 0: element 1 of "image": expected int, got string`},
		{"negative thickness", []any{[]any{"line", 0.0, 0.0, 1.0, 1.0, int64(0), int64(0), int64(0), -1.0}}, `command 0: element 8 of "line": thickness must not be negative, got -1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeFrame(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestDecodeFrame`
Expected: FAIL (undefined: decodeFrame)

- [ ] **Step 3: Write minimal implementation**

```go
// commands.go — o protocolo de frame: any[] de any[] (tag + argumentos) →
// []command tipado. Sem dependência de Ebiten, para ser testável sem janela.
package main

import "fmt"

type cmdKind int

const (
	cmdClear cmdKind = iota
	cmdRect
	cmdCircle
	cmdLine
	cmdText
	cmdImage
)

type color3 struct{ R, G, B uint8 }

// command é um desenho decodificado. Campos por tag:
//   clear:  color
//   rect:   x y w h color thickness (0 = preenchido)
//   circle: x y (centro) w (raio) color thickness
//   line:   x y (início) w h (fim) color thickness
//   text:   text x y size color
//   image:  image x y scale angle (graus)
type command struct {
	kind                                        cmdKind
	color                                       color3
	x, y, w, h, thickness, size, scale, angle   float64
	text                                        string
	image                                       int64
}

// arity é o tamanho exato (tag incluída) de cada comando.
var arity = map[string]int{
	"clear": 4, "rect": 9, "circle": 8, "line": 9, "text": 8, "image": 6,
}

func decodeFrame(raw []any) ([]command, error) {
	cmds := make([]command, 0, len(raw))
	for i, item := range raw {
		c, err := decodeCommand(item)
		if err != nil {
			return nil, fmt.Errorf("command %d: %w", i, err)
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

func decodeCommand(item any) (command, error) {
	parts, ok := item.([]any)
	if !ok {
		return command{}, fmt.Errorf("expected an array, got %s", typeName(item))
	}
	if len(parts) == 0 {
		return command{}, fmt.Errorf("empty command")
	}
	tag, ok := parts[0].(string)
	if !ok {
		return command{}, fmt.Errorf("tag must be a string, got %s", typeName(parts[0]))
	}
	want, ok := arity[tag]
	if !ok {
		return command{}, fmt.Errorf("unknown tag %q", tag)
	}
	if len(parts) != want {
		return command{}, fmt.Errorf("%q expects %d elements, got %d", tag, want, len(parts))
	}
	r := reader{tag: tag, parts: parts}
	var c command
	switch tag {
	case "clear":
		c.kind = cmdClear
		c.color = r.color(1)
	case "rect":
		c.kind = cmdRect
		c.x, c.y, c.w, c.h = r.num(1), r.num(2), r.num(3), r.num(4)
		c.color = r.color(5)
		c.thickness = r.thickness(8)
	case "circle":
		c.kind = cmdCircle
		c.x, c.y, c.w = r.num(1), r.num(2), r.num(3)
		c.color = r.color(4)
		c.thickness = r.thickness(7)
	case "line":
		c.kind = cmdLine
		c.x, c.y, c.w, c.h = r.num(1), r.num(2), r.num(3), r.num(4)
		c.color = r.color(5)
		c.thickness = r.thickness(8)
	case "text":
		c.kind = cmdText
		c.text = r.str(1)
		c.x, c.y, c.size = r.num(2), r.num(3), r.num(4)
		c.color = r.color(5)
	case "image":
		c.kind = cmdImage
		c.image = r.integer(1)
		c.x, c.y, c.scale, c.angle = r.num(2), r.num(3), r.num(4), r.num(5)
	}
	if r.err != nil {
		return command{}, r.err
	}
	return c, nil
}

// reader lê os elementos de um comando guardando o primeiro erro; o índice
// nas mensagens é o do elemento (1 = primeiro argumento após a tag).
type reader struct {
	tag   string
	parts []any
	err   error
}

func (r *reader) fail(i int, format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("element %d of %q: %s", i, r.tag, fmt.Sprintf(format, args...))
	}
}

func (r *reader) num(i int) float64 {
	switch v := r.parts[i].(type) {
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case float64:
		return v
	}
	r.fail(i, "expected number, got %s", typeName(r.parts[i]))
	return 0
}

func (r *reader) integer(i int) int64 {
	switch v := r.parts[i].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	}
	r.fail(i, "expected int, got %s", typeName(r.parts[i]))
	return 0
}

func (r *reader) str(i int) string {
	s, ok := r.parts[i].(string)
	if !ok {
		r.fail(i, "expected string, got %s", typeName(r.parts[i]))
	}
	return s
}

func (r *reader) thickness(i int) float64 {
	t := r.num(i)
	if t < 0 {
		r.fail(i, "thickness must not be negative, got %g", t)
	}
	return t
}

// color lê r, g, b nos elementos i, i+1, i+2.
func (r *reader) color(i int) color3 {
	var out [3]uint8
	for k := 0; k < 3; k++ {
		idx := i + k
		var n int64
		switch v := r.parts[idx].(type) {
		case int64:
			n = v
		case int:
			n = int64(v)
		default:
			r.fail(idx, "color component must be an int, got %s", typeName(v))
			return color3{}
		}
		if n < 0 || n > 255 {
			r.fail(idx, "color component out of range 0..255, got %d", n)
			return color3{}
		}
		out[k] = uint8(n)
	}
	return color3{out[0], out[1], out[2]}
}

// typeName nomeia um valor NXB decodificado no vocabulário do Noxy.
func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case int64, int:
		return "int"
	case float64:
		return "float"
	case string:
		return "string"
	case []byte:
		return "bytes"
	case []any:
		return "array"
	case map[string]any, map[int64]any, map[any]any:
		return "map"
	}
	return fmt.Sprintf("%T", v)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestDecodeFrame -v`
Expected: PASS (3 testes; a suíte de erros com 11 subtestes)

- [ ] **Step 5: Commit**

```bash
git add commands.go commands_test.go go.mod go.sum
git commit -m "feat(commands): decodificador do protocolo de frame (any[] -> []command)"
```

---

### Task 2: Nomes de tecla e botão

**Files:**
- Create: `keys.go`, `keys_test.go`

**Interfaces:**
- Produces: `func keyName(k ebiten.Key) string`, `func mouseButtonName(b ebiten.MouseButton) string`.

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestKeyName(t *testing.T) {
	cases := map[ebiten.Key]string{
		ebiten.KeyA:            "a",
		ebiten.KeyZ:            "z",
		ebiten.KeyDigit0:       "0",
		ebiten.KeyDigit9:       "9",
		ebiten.KeyArrowLeft:    "left",
		ebiten.KeyArrowRight:   "right",
		ebiten.KeyArrowUp:      "up",
		ebiten.KeyArrowDown:    "down",
		ebiten.KeySpace:        "space",
		ebiten.KeyEnter:        "enter",
		ebiten.KeyEscape:       "escape",
		ebiten.KeyTab:          "tab",
		ebiten.KeyBackspace:    "backspace",
		ebiten.KeyShiftLeft:    "shift",
		ebiten.KeyShiftRight:   "shift",
		ebiten.KeyControlLeft:  "ctrl",
		ebiten.KeyControlRight: "ctrl",
		ebiten.KeyAltLeft:      "alt",
		ebiten.KeyAltRight:     "alt",
		ebiten.KeyF1:           "f1",
		ebiten.KeyF12:          "f12",
		ebiten.KeyNumpad5:      "numpad5",
	}
	for k, want := range cases {
		if got := keyName(k); got != want {
			t.Errorf("keyName(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestMouseButtonName(t *testing.T) {
	if mouseButtonName(ebiten.MouseButtonLeft) != "left" ||
		mouseButtonName(ebiten.MouseButtonRight) != "right" ||
		mouseButtonName(ebiten.MouseButtonMiddle) != "middle" {
		t.Fatal("mouse button names")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestKeyName|TestMouseButtonName'`
Expected: FAIL (undefined: keyName)

- [ ] **Step 3: Write minimal implementation**

```go
// keys.go — nomes de tecla e botão no vocabulário do wrapper ("left",
// "space", "a", "0", "shift"...), derivados de ebiten.Key.String().
package main

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

func keyName(k ebiten.Key) string {
	s := k.String()
	switch {
	case strings.HasPrefix(s, "Arrow"):
		return strings.ToLower(strings.TrimPrefix(s, "Arrow"))
	case strings.HasPrefix(s, "Digit"):
		return strings.TrimPrefix(s, "Digit")
	case strings.HasPrefix(s, "Shift"):
		return "shift"
	case strings.HasPrefix(s, "Control"):
		return "ctrl"
	case strings.HasPrefix(s, "Alt"):
		return "alt"
	case strings.HasPrefix(s, "Meta"):
		return "meta"
	}
	return strings.ToLower(s)
}

func mouseButtonName(b ebiten.MouseButton) string {
	switch b {
	case ebiten.MouseButtonLeft:
		return "left"
	case ebiten.MouseButtonRight:
		return "right"
	case ebiten.MouseButtonMiddle:
		return "middle"
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestKeyName|TestMouseButtonName' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add keys.go keys_test.go go.mod go.sum
git commit -m "feat(keys): nomes de tecla e botao do mouse no vocabulario do wrapper"
```

---

### Task 3: Engine headless (frame pendente, snapshot, ciclo de vida)

**Files:**
- Create: `engine.go`, `engine_test.go`

**Interfaces:**
- Consumes: `command`, `decodeFrame` (Task 1).
- Produces:
  - `type inputReader interface{ keysDown() []string; keysPressed() []string; mouse() (x, y int, down, pressed []string); windowClosing() bool }`
  - `type inputSnapshot struct{...}` + `func (s inputSnapshot) toMap() map[string]any`
  - `type initRequest struct{ width, height int; title string }`
  - `type engine struct` com `newEngine() *engine`, `handleInit(ctx, w, h int64, title string) (any, error)`, `handleFlip(ctx, cmds []any) (map[string]any, error)`, `handleLoadImage(ctx, path string) (map[string]any, error)`, `handleQuit(ctx) (any, error)`, `tick(in inputReader) (terminate bool)`, `currentFrame() []command`, `image(id int64) (*imageEntry, error)`, `markReady()`, `markFailed(err error)`, campos `initReq chan initRequest`, `width, height int`.
  - `type imageEntry struct{ src image.Image; tex *ebiten.Image }` — `tex` é preenchido pelo render (Task 4); `engine.go` importa `ebiten` só por esse tipo.

- [ ] **Step 1: Write the failing test**

```go
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
	// espera o frame ficar pendente, então simula um Update
	deadline := time.Now().Add(time.Second)
	for !e.hasPendingFrame() {
		if time.Now().After(deadline) {
			t.Fatal("frame never became pending")
		}
		time.Sleep(time.Millisecond)
	}
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

func TestFlipHonoursContextCancel(t *testing.T) {
	e := startedEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.handleFlip(ctx, nil)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

func TestWindowCloseMarksClosed(t *testing.T) {
	e := startedEngine(t)
	res := make(chan map[string]any, 1)
	go func() {
		s, _ := e.handleFlip(context.Background(), nil)
		res <- s
	}()
	for !e.hasPendingFrame() {
		time.Sleep(time.Millisecond)
	}
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'Test(Calls|Init|Flip|Window|Quit|LoadImage|Snapshot)'`
Expected: FAIL (undefined: newEngine, ...)

- [ ] **Step 3: Write minimal implementation**

```go
// engine.go — estado compartilhado entre a goroutine do SDK (handlers) e o
// loop do Ebiten (tick/Draw na thread principal). Tudo aqui roda sem janela:
// o Ebiten entra só por inputReader (main.go) e pelo render (render.go).
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

// inputReader é o que o tick lê a cada Update; main.go dá a implementação
// sobre o Ebiten, os testes dão uma fake.
type inputReader interface {
	keysDown() []string
	keysPressed() []string
	mouse() (x, y int, down, pressed []string)
	windowClosing() bool
}

// inputSnapshot é o retorno de game_flip.
type inputSnapshot struct {
	Closed                  bool
	DT                      float64
	KeysDown, KeysPressed   []string
	MouseX, MouseY          int
	MouseDown, MousePressed []string
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
	}
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
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
	quit    bool // game_quit chamado
	closed  bool // usuário fechou a janela

	width, height int
	initReq       chan initRequest // main() espera aqui antes de RunGame
	readyCh       chan struct{}    // fechado por markReady
	failedCh      chan struct{}    // fechado por markFailed
	failure       error

	pending  *pendingFrame
	current  []command
	lastTick time.Time

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

// handleFlip: game_flip(cmds) -> snapshot. Valida o frame inteiro, publica-o
// e espera o tick que o consumir.
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
func (e *engine) tick(in inputReader) (terminate bool) {
	e.markReady()
	e.mu.Lock()
	if e.quit {
		e.mu.Unlock()
		return true
	}
	if in.windowClosing() {
		e.closed = true
	}
	frame := e.pending
	e.pending = nil
	if frame != nil {
		e.current = frame.cmds
	}
	now := time.Now()
	dt := now.Sub(e.lastTick).Seconds()
	closed := e.closed
	e.mu.Unlock()
	if frame == nil {
		return false
	}
	e.mu.Lock()
	e.lastTick = now
	e.mu.Unlock()
	mx, my, mdown, mpressed := in.mouse()
	frame.reply <- inputSnapshot{
		Closed:       closed,
		DT:           dt,
		KeysDown:     in.keysDown(),
		KeysPressed:  in.keysPressed(),
		MouseX:       mx,
		MouseY:       my,
		MouseDown:    mdown,
		MousePressed: mpressed,
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -race -v`
Expected: PASS em todos (Task 1–3)

- [ ] **Step 5: Commit**

```bash
git add engine.go engine_test.go go.mod go.sum
git commit -m "feat(engine): frame pendente, snapshot de input e ciclo de vida (headless)"
```

---

### Task 4: Render, fonte, main e manifesto — binário compilando

**Files:**
- Create: `render.go`, `font.go`, `main.go`, `noxy_ext.toml`, `noxy.mod`

**Interfaces:**
- Consumes: `engine` (Task 3), `keyName`/`mouseButtonName` (Task 2), `command` (Task 1).
- Produces: `type game struct{ e *engine }` implementando `ebiten.Game`; `type ebitenInput struct{}` implementando `inputReader`; `func fontFace(size float64) text.Face`.

- [ ] **Step 1: Write `font.go`**

```go
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
```

- [ ] **Step 2: Write `render.go`**

```go
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
		entry, err := g.e.image(c.image)
		if err != nil {
			return // validado no flip? não: o id só é conferido aqui; desenho ignorado
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
```

Nota: em `cmdImage`, um id desconhecido é ignorado no desenho. Para que o
erro apareça ao usuário, `handleFlip` valida os ids **antes** de publicar —
ver Step 3 (ajuste em `engine.go`).

- [ ] **Step 3: Validar ids de imagem no flip (`engine.go`)**

Em `handleFlip`, logo após `decodeFrame` e antes de publicar, dentro do lock:

```go
	for i, c := range cmds {
		if c.kind == cmdImage {
			if _, ok := e.images[c.image]; !ok {
				e.mu.Unlock()
				return nil, fmt.Errorf("command %d: unknown image %d (not returned by load_image)", i, c.image)
			}
		}
	}
```

E o teste correspondente em `engine_test.go`:

```go
func TestFlipRejectsUnknownImage(t *testing.T) {
	e := startedEngine(t)
	_, err := e.handleFlip(context.Background(), []any{[]any{"image", int64(42), 0.0, 0.0, 1.0, 0.0}})
	if err == nil || !strings.Contains(err.Error(), "command 0: unknown image 42") {
		t.Fatalf("got %v", err)
	}
}
```

Run: `go test ./... -run TestFlipRejectsUnknownImage` → PASS.

- [ ] **Step 4: Write `main.go`**

```go
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
```

- [ ] **Step 5: Write `noxy_ext.toml` and `noxy.mod`**

`noxy_ext.toml`:

```toml
name = "game"
abi = 1
kind = "process"
min_noxy = "0.23.0"
concurrency = "single"          # um jogo, um loop: uma chamada por vez
capabilities = ["display", "fs"] # abre uma janela; load_image le arquivos

[binaries]                      # release assets; noxy --get baixa o do seu OS/arch
windows-amd64 = "noxy-plugin-game-windows-amd64.exe"
linux-amd64   = "noxy-plugin-game-linux-amd64"
darwin-amd64  = "noxy-plugin-game-darwin-amd64"
darwin-arm64  = "noxy-plugin-game-darwin-arm64"

[[export]]
name = "game_init"              # (width, height, title): abre a janela
params = ["int", "int", "string"]
returns = "void"
stateful = true

[[export]]
name = "game_flip"              # (commands): mostra o frame, espera o proximo tick, devolve o input
params = ["any[]"]
returns = "map[string]any"
timeout_ms = 10000

[[export]]
name = "game_load_image"        # (path) -> {"id", "width", "height"}
params = ["string"]
returns = "map[string]any"
stateful = true

[[export]]
name = "game_quit"
params = []
returns = "void"
```

`noxy.mod`:

```
module noxy_game_engine

noxy v0.23.0
```

- [ ] **Step 6: Build, vet, test**

Run: `go mod tidy && go build ./... && go vet ./... && go test ./... -race`
Expected: build limpo, PASS. Depois `go build -o bin/noxy-plugin-game-windows-amd64.exe .` gera o binário (bin/ está no .gitignore).

- [ ] **Step 7: Commit**

```bash
git add render.go font.go main.go noxy_ext.toml noxy.mod engine.go engine_test.go go.mod go.sum
git commit -m "feat(plugin): render Ebiten, main com loop na thread principal, manifesto da extensao"
```

---

### Task 5: Wrapper Noxy e smoke test de integração

**Files:**
- Create: `noxy_game_engine.nx`, `examples/smoke.nx`

**Interfaces:**
- Consumes: natives `game_init`, `game_flip`, `game_load_image`, `game_quit` (Task 4).
- Produces: API pública do módulo — `init`, `running`, `stop`, `quit`, `flip`, `delta`, `clear`, `draw_rect`, `draw_rect_outline`, `draw_circle`, `draw_circle_outline`, `draw_line`, `draw_text`, `load_image`, `draw_image`, `draw_image_ex`, `key_down`, `key_pressed`, `mouse_pos`, `mouse_down`, `mouse_pressed`, `rgb`, structs `Color`, `Image`, `Point`, constantes `BLACK WHITE RED GREEN BLUE YELLOW`.

- [ ] **Step 1: Write `noxy_game_engine.nx`**

```noxy
// noxy_game_engine.nx — typed wrapper over the `game` process extension.
//
// The natives (game_init, game_flip, game_load_image, game_quit) are
// registered by the VM from noxy_ext.toml when this module is imported; the
// binary in bin/ starts on the first call. Draw calls only append to a
// per-frame command list; flip() sends the whole frame in ONE call, waits
// for the next 60 Hz tick and refreshes the input snapshot read by key_down,
// mouse_pos, running, ... A failure inside the extension is a runtime error
// (`extension 'game' failed: <message>`), capturable with call_result.

struct Color
    r: int
    g: int
    b: int
end

struct Image
    id: int
    width: int
    height: int
end

struct Point
    x: int
    y: int
end

let BLACK: Color = Color(0, 0, 0)
let WHITE: Color = Color(255, 255, 255)
let RED: Color = Color(255, 0, 0)
let GREEN: Color = Color(0, 255, 0)
let BLUE: Color = Color(0, 0, 255)
let YELLOW: Color = Color(255, 255, 0)

func rgb(r: int, g: int, b: int) -> Color
    return Color(r, g, b)
end

// module state: the frame being built and the last input snapshot
let _cmds: any[] = []
let _running: bool = false
let _dt: float = 0.0
let _keys_down: string[] = []
let _keys_pressed: string[] = []
let _mouse_x: int = 0
let _mouse_y: int = 0
let _mouse_down: string[] = []
let _mouse_pressed: string[] = []

// init opens a width x height window titled `title`. Raises if called twice
// or if the window cannot be opened.
func init(width: int, height: int, title: string) -> void
    game_init(width, height, title)
    _running = true
    _cmds = []
end

// running is true from init until the window is closed, stop() or quit().
func running() -> bool
    return _running
end

// stop leaves the game loop (running() becomes false); the window stays
// until quit() or the end of the program.
func stop() -> void
    _running = false
end

// quit closes the window. Idempotent.
func quit() -> void
    _running = false
    game_quit()
end

// flip shows the frame built since the last flip, waits for the next tick
// (60 Hz) and refreshes the input snapshot. Raises on an invalid frame.
func flip() -> void
    let snap: map[string, any] = game_flip(_cmds)
    _cmds = []
    let closed: bool = snap["closed"]
    if closed then
        _running = false
    end
    let dt: float = snap["dt"]
    _dt = dt
    let kd: string[] = snap["keys_down"]
    _keys_down = kd
    let kp: string[] = snap["keys_pressed"]
    _keys_pressed = kp
    let mx: int = snap["mouse_x"]
    _mouse_x = mx
    let my: int = snap["mouse_y"]
    _mouse_y = my
    let md: string[] = snap["mouse_down"]
    _mouse_down = md
    let mp: string[] = snap["mouse_pressed"]
    _mouse_pressed = mp
end

// delta is the time in seconds between the last two flips.
func delta() -> float
    return _dt
end

// ---- drawing (queued until flip) ----

func clear(c: Color) -> void
    append(ref _cmds, ["clear", c.r, c.g, c.b])
end

func draw_rect(x: float, y: float, w: float, h: float, c: Color) -> void
    append(ref _cmds, ["rect", x, y, w, h, c.r, c.g, c.b, 0])
end

func draw_rect_outline(x: float, y: float, w: float, h: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["rect", x, y, w, h, c.r, c.g, c.b, thickness])
end

func draw_circle(cx: float, cy: float, radius: float, c: Color) -> void
    append(ref _cmds, ["circle", cx, cy, radius, c.r, c.g, c.b, 0])
end

func draw_circle_outline(cx: float, cy: float, radius: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["circle", cx, cy, radius, c.r, c.g, c.b, thickness])
end

func draw_line(x1: float, y1: float, x2: float, y2: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["line", x1, y1, x2, y2, c.r, c.g, c.b, thickness])
end

// draw_text draws `s` with its top-left corner at (x, y), `size` px high,
// in the built-in Go Regular font.
func draw_text(s: string, x: float, y: float, size: int, c: Color) -> void
    append(ref _cmds, ["text", s, x, y, size, c.r, c.g, c.b])
end

// load_image decodes a PNG or JPEG (path relative to the working directory).
// Raises if the file is missing or not an image.
func load_image(path: string) -> Image
    let info: map[string, any] = game_load_image(path)
    let id: int = info["id"]
    let w: int = info["width"]
    let h: int = info["height"]
    return Image(id, w, h)
end

// draw_image draws img with its top-left corner at (x, y), unscaled.
func draw_image(img: Image, x: float, y: float) -> void
    append(ref _cmds, ["image", img.id, x, y, 1.0, 0.0])
end

// draw_image_ex scales img by `scale` and rotates it `angle_deg` degrees
// around the center of the scaled image; (x, y) is the top-left corner.
func draw_image_ex(img: Image, x: float, y: float, scale: float, angle_deg: float) -> void
    append(ref _cmds, ["image", img.id, x, y, scale, angle_deg])
end

// ---- input (snapshot taken by the last flip) ----

func _has(xs: string[], k: string) -> bool
    for x in xs do
        if x == k then
            return true
        end
    end
    return false
end

// key_down: the key is held. Names: "a".."z", "0".."9", "left" "right" "up"
// "down", "space" "enter" "escape" "tab" "backspace", "shift" "ctrl" "alt",
// "f1".."f12".
func key_down(key: string) -> bool
    return _has(_keys_down, key)
end

// key_pressed: the key went down during the last frame.
func key_pressed(key: string) -> bool
    return _has(_keys_pressed, key)
end

func mouse_pos() -> Point
    return Point(_mouse_x, _mouse_y)
end

// mouse_down / mouse_pressed: button is "left", "right" or "middle".
func mouse_down(button: string) -> bool
    return _has(_mouse_down, button)
end

func mouse_pressed(button: string) -> bool
    return _has(_mouse_pressed, button)
end
```

- [ ] **Step 2: Write `examples/smoke.nx`**

```noxy
// smoke.nx — opens a window, draws a few frames and quits. Run it to check
// the installation: `noxy examples/smoke.nx` prints "ok" and exits 0.
use github_com.estevaofon.noxy_game_engine as game

game.init(320, 240, "noxy_game_engine smoke")
let frames = 0
while game.running() && frames < 30 do
    game.clear(game.rgb(20, 20, 40))
    game.draw_rect(10.0 + frames * 4.0, 60.0, 50.0, 50.0, game.RED)
    game.draw_rect_outline(200.0, 60.0, 50.0, 50.0, game.YELLOW, 2.0)
    game.draw_circle(160.0, 160.0, 20.0, game.GREEN)
    game.draw_circle_outline(160.0, 160.0, 30.0, game.WHITE, 1.0)
    game.draw_line(0.0, 0.0, 320.0, 240.0, game.BLUE, 2.0)
    game.draw_text(f"frame {frames} dt={game.delta()}", 8.0, 8.0, 14, game.WHITE)
    game.flip()
    frames = frames + 1
end
game.quit()
print("ok")
```

- [ ] **Step 3: Instalar o checkout no projeto Noxy e rodar**

```powershell
go build -o bin/noxy-plugin-game-windows-amd64.exe .
# link do checkout como pacote instalado (uma vez):
New-Item -ItemType Junction -Path C:\Users\sandr\Documents\noxy\noxy_libs\github_com\estevaofon\noxy_game_engine -Target C:\Users\sandr\Documents\noxy_game_engine
cd C:\Users\sandr\Documents\noxy
.\noxy.exe ..\noxy_game_engine\examples\smoke.nx
```

Expected: uma janela aparece por ~0,5 s (30 frames a 60 Hz), o programa imprime `ok` e sai com 0. (Aviso de trust-on-first-use por não haver entrada em `noxy.sum` é esperado.)

Se `let kd: string[] = snap["keys_down"]` falhar em runtime (array de `any` vindo da extensão não aceito como `string[]`), trocar por uma conversão explícita em `_has`: iterar `snap["keys_down"]` como `any[]` e comparar com `to_str(x)`.

- [ ] **Step 4: Commit**

```bash
git add noxy_game_engine.nx examples/smoke.nx
git commit -m "feat(wrapper): API Noxy estilo pygame e smoke test da instalacao"
```

---

### Task 6: Exemplos jogáveis

**Files:**
- Create: `examples/bouncing_ball.nx`, `examples/pong.nx`

**Interfaces:**
- Consumes: a API do wrapper (Task 5).

- [ ] **Step 1: Write `examples/bouncing_ball.nx`**

```noxy
// bouncing_ball.nx — a ball bouncing off the window edges. Escape quits.
use github_com.estevaofon.noxy_game_engine as game

let W = 640.0
let H = 480.0
game.init(640, 480, "Bouncing ball")

let x = 320.0
let y = 240.0
let vx = 180.0   // px per second
let vy = 140.0
let r = 16.0

while game.running() do
    if game.key_pressed("escape") then
        game.stop()
    end
    let dt = game.delta()
    x = x + vx * dt
    y = y + vy * dt
    if x - r < 0.0 || x + r > W then
        vx = -vx
    end
    if y - r < 0.0 || y + r > H then
        vy = -vy
    end

    game.clear(game.BLACK)
    game.draw_circle(x, y, r, game.YELLOW)
    game.draw_text("Escape to quit", 10.0, 10.0, 16, game.WHITE)
    game.flip()
end
game.quit()
```

- [ ] **Step 2: Write `examples/pong.nx`**

```noxy
// pong.nx — two paddles (W/S and Up/Down), first to 5 wins. Escape quits.
use github_com.estevaofon.noxy_game_engine as game

let W = 640.0
let H = 480.0
let PADDLE_H = 80.0
let PADDLE_W = 12.0
let SPEED = 300.0

game.init(640, 480, "Pong")

let left_y = H / 2.0 - PADDLE_H / 2.0
let right_y = left_y
let ball_x = W / 2.0
let ball_y = H / 2.0
let ball_vx = 220.0
let ball_vy = 160.0
let score_l = 0
let score_r = 0

func reset_ball() -> void
    ball_x = W / 2.0
    ball_y = H / 2.0
    ball_vx = -ball_vx
    ball_vy = 160.0
end

func clamp(v: float, lo: float, hi: float) -> float
    if v < lo then return lo end
    if v > hi then return hi end
    return v
end

while game.running() do
    if game.key_pressed("escape") then
        game.stop()
    end
    let dt = game.delta()

    if game.key_down("w") then left_y = left_y - SPEED * dt end
    if game.key_down("s") then left_y = left_y + SPEED * dt end
    if game.key_down("up") then right_y = right_y - SPEED * dt end
    if game.key_down("down") then right_y = right_y + SPEED * dt end
    left_y = clamp(left_y, 0.0, H - PADDLE_H)
    right_y = clamp(right_y, 0.0, H - PADDLE_H)

    ball_x = ball_x + ball_vx * dt
    ball_y = ball_y + ball_vy * dt
    if ball_y < 0.0 || ball_y > H then
        ball_vy = -ball_vy
    end
    // left paddle at x = 20, right paddle at x = W - 32
    if ball_x < 32.0 && ball_y > left_y && ball_y < left_y + PADDLE_H then
        ball_vx = -ball_vx * 1.05
        ball_x = 32.0
    end
    if ball_x > W - 32.0 && ball_y > right_y && ball_y < right_y + PADDLE_H then
        ball_vx = -ball_vx * 1.05
        ball_x = W - 32.0
    end
    if ball_x < 0.0 then
        score_r = score_r + 1
        reset_ball()
    end
    if ball_x > W then
        score_l = score_l + 1
        reset_ball()
    end

    game.clear(game.BLACK)
    game.draw_line(W / 2.0, 0.0, W / 2.0, H, game.rgb(80, 80, 80), 2.0)
    game.draw_rect(20.0, left_y, PADDLE_W, PADDLE_H, game.WHITE)
    game.draw_rect(W - 32.0, right_y, PADDLE_W, PADDLE_H, game.WHITE)
    game.draw_circle(ball_x, ball_y, 8.0, game.YELLOW)
    game.draw_text(f"{score_l}", W / 2.0 - 60.0, 20.0, 32, game.WHITE)
    game.draw_text(f"{score_r}", W / 2.0 + 40.0, 20.0, 32, game.WHITE)
    if score_l >= 5 || score_r >= 5 then
        game.draw_text("Game over - Escape to quit", W / 2.0 - 120.0, H / 2.0, 20, game.RED)
    end
    game.flip()
end
game.quit()
```

- [ ] **Step 3: Rodar os dois à mão**

```powershell
cd C:\Users\sandr\Documents\noxy
.\noxy.exe ..\noxy_game_engine\examples\bouncing_ball.nx
.\noxy.exe ..\noxy_game_engine\examples\pong.nx
```

Expected: janelas jogáveis; Escape encerra com exit 0. Se um exemplo não compilar (regra da linguagem), ajustar o exemplo — a spec da linguagem manda.

- [ ] **Step 4: Commit**

```bash
git add examples/bouncing_ball.nx examples/pong.nx
git commit -m "docs(examples): bola quicando e pong"
```

---

### Task 7: README, script de release e workflow

**Files:**
- Create: `README.md`, `release/build.sh`, `.github/workflows/release.yml`

- [ ] **Step 1: Write `release/build.sh`** (compila só a plataforma corrente — Ebiten precisa de cgo fora do Windows)

```sh
#!/usr/bin/env sh
# Builds this platform's plugin binary into dist/ and its sha256 line.
# The release workflow runs it on one runner per OS and merges checksums.txt.
set -eu
NAME="${1:?usage: build.sh <extension-name> [GOARCH]}"
ARCH="${2:-$(go env GOARCH)}"
OS="$(go env GOOS)"
ext=""; [ "$OS" = windows ] && ext=".exe"
mkdir -p dist
GOARCH="$ARCH" go build -trimpath -ldflags=-s -o "dist/noxy-plugin-$NAME-$OS-$ARCH$ext" .
(cd dist && sha256sum -- "noxy-plugin-$NAME-$OS-$ARCH$ext" > "checksums-$OS-$ARCH.txt")
echo "dist/noxy-plugin-$NAME-$OS-$ARCH$ext"
```

- [ ] **Step 2: Write `.github/workflows/release.yml`**

```yaml
# Release workflow for the noxy_game_engine process extension. Ebiten needs
# cgo on Linux and macOS, so each OS builds natively; the last job merges the
# per-platform checksums into the checksums.txt that `noxy --get` expects.
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  build:
    strategy:
      matrix:
        include:
          - os: windows-latest
            arch: amd64
          - os: ubuntu-latest
            arch: amd64
          - os: macos-latest
            arch: arm64
          - os: macos-latest
            arch: amd64
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - if: runner.os == 'Linux'
        run: sudo apt-get update && sudo apt-get install -y libgl1-mesa-dev xorg-dev libasound2-dev
      - run: sh release/build.sh game ${{ matrix.arch }}
        shell: bash
      - uses: actions/upload-artifact@v4
        with:
          name: dist-${{ matrix.os }}-${{ matrix.arch }}
          path: dist/*

  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true
      - run: cd dist && cat checksums-*.txt > checksums.txt && rm checksums-*.txt && ls -l
      - uses: softprops/action-gh-release@v2
        with:
          files: dist/*
```

- [ ] **Step 3: Write `README.md`** — mesma estrutura do `noxy_dynamodb`: título, o que é, instalação (`noxy --get github.com/estevaofon/noxy_game_engine`), exemplo completo (o `bouncing_ball`), tabela da API (função | descrição), nomes de tecla, erros (`call_result`), notas de plataforma (cgo em Linux/macOS: só os binários listados; Linux precisa de X11/GL em runtime), seção Development (build local, junction em `noxy_libs`, `go test`), Releasing (tag `vX.Y.Z`). Conteúdo completo no Step 3 da execução — escrever a partir da spec §"Uso alvo", §"Snapshot", §"Plataformas".

- [ ] **Step 4: Verificação final**

Run: `go build ./... && go vet ./... && go test ./... -race && sh release/build.sh game`
Expected: PASS; `dist/noxy-plugin-game-windows-amd64.exe` e `dist/checksums-windows-amd64.txt` criados. Rodar `examples/smoke.nx` uma última vez.

- [ ] **Step 5: Commit**

```bash
git add README.md release/build.sh .github/workflows/release.yml
git commit -m "docs(readme): instalacao, API e release por plataforma"
```
