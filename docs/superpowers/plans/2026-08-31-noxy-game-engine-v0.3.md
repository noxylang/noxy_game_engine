# noxy_game_engine v0.3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar o pacote v0.3 da issue #1: fonte customizada, controle
de janela (fullscreen e FPS), input que faltava (roda do mouse, texto
digitado, gamepad), polígonos preenchidos, e câmera + colisão no wrapper.

**Architecture:** `font.go` vira um registro de fontes (id 0 = embutida) com
cache por par (fonte, tamanho); o comando `text` do protocolo carrega o id.
As operações de janela são pedidos guardados no engine e aplicados pelo
`tick` na thread principal, através de uma interface `platform` (que estende
`inputReader`) — assim `engine.go` continua testável sem janela. Gamepad e
polígono seguem os padrões já existentes: tabela de nomes em arquivo próprio
e comando de aridade fixa (a lista de pontos é um array aninhado). Câmera e
colisão são Noxy puro no wrapper.

**Tech Stack:** Go 1.25, Ebiten v2.9.10 (`vector` com `Path`/`FillPath`,
`text/v2`, `inpututil` incl. gamepad padrão), SDK noxyplugin v0.1.0, wrapper
Noxy.

**Spec:** `docs/superpowers/specs/2026-08-31-noxy-game-engine-v0.3-design.md`

## Global Constraints

- Commits em português no formato `tipo(escopo): descrição` (padrão do repo).
- Verificação de toda task: `go build ./... && go vet ./... && go test -race ./... -count=1` na raiz do repo.
- v0.3 é **aditivo no wrapper**: nenhuma assinatura pública `.nx` existente muda. O protocolo interno muda (comando `text` ganha o id da fonte; `game_text_width` ganha o parâmetro).
- Testes Go são headless: nunca criar janela, `audio.Context` nem depender de gamepad físico.
- Fonte id **0 = Go Regular embutida**; ids carregados começam em 1.
- Numeração de gamepad = **posição na lista de controles com layout padrão**, não o id bruto do Ebiten.
- Nada de asset de fonte (TTF/OTF) commitado no repositório — licença. Validação de `load_font` é local, com fonte do sistema, em script descartável.
- Binário local (Windows): `go build -o bin/noxy-plugin-game-windows-amd64.exe .`; exemplos rodam via junction: `cd /c/Users/sandr/Documents/noxy && ./noxy.exe ../noxy_game_engine/examples/<x>.nx`.
- A tag `v0.3.0` só é criada com confirmação explícita do usuário (fora deste plano).

---

### Task 1: Registro de fontes

**Files:**
- Modify: `font.go` (registro, cache por par, textWidth/drawText com id)
- Test: `font_test.go`

**Interfaces:**
- Consumes: nada de tasks anteriores.
- Produces: `loadFont(path string) (int64, error)`; `fontExists(id int64) bool`; `textWidth(s string, size float64, fontID int64) (float64, error)`; `drawText(dst *ebiten.Image, s string, size float64, fontID int64, op *text.DrawOptions) error`. A Task 2 chama `loadFont`/`fontExists`/`textWidth`, a Task 6 não toca aqui, e `render.go` passa a chamar `drawText` com o id do comando.

- [ ] **Step 1: Reescrever os testes de fonte**

Substituir `font_test.go` inteiro (a assinatura de `textWidth` muda, então os
testes atuais não compilam mais):

```go
// font_test.go — registro e medida de fontes; headless (sem janela).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTextWidthBuiltin(t *testing.T) {
	w, err := textWidth("", 16, 0)
	if err != nil || w != 0 {
		t.Fatalf("empty string: want 0, got %g (%v)", w, err)
	}
	ab, err := textWidth("ab", 16, 0)
	if err != nil {
		t.Fatal(err)
	}
	abc, _ := textWidth("abc", 16, 0)
	if !(ab > 0 && abc > ab) {
		t.Fatalf("want 0 < %g < %g", ab, abc)
	}
	big, _ := textWidth("ab", 32, 0)
	if big <= ab {
		t.Fatalf("size 32 (%g) should be wider than 16 (%g)", big, ab)
	}
}

func TestTextWidthUnknownFont(t *testing.T) {
	_, err := textWidth("hi", 16, 99)
	if err == nil || !strings.Contains(err.Error(), "unknown font 99 (not returned by load_font)") {
		t.Fatalf("want unknown font error, got %v", err)
	}
}

func TestLoadFontErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadFont(filepath.Join(dir, "nope.ttf")); err == nil {
		t.Fatal("want error for missing file")
	}
	bad := filepath.Join(dir, "bad.ttf")
	if err := os.WriteFile(bad, []byte("not a font at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFont(bad); err == nil {
		t.Fatal("want error for a file that is not a font")
	}
}

func TestFontExists(t *testing.T) {
	if !fontExists(0) {
		t.Fatal("id 0 (built-in) must exist")
	}
	if fontExists(4242) {
		t.Fatal("unknown id must not exist")
	}
}

func TestFontFaceCachedPerPair(t *testing.T) {
	fontMu.Lock()
	before := len(fontFaces)
	fontMu.Unlock()
	textWidth("x", 21, 0) // tamanho improvável de já estar no cache
	textWidth("y", 21, 0)
	fontMu.Lock()
	after := len(fontFaces)
	fontMu.Unlock()
	if after != before+1 {
		t.Fatalf("want exactly one new face, got %d -> %d", before, after)
	}
}

func TestFontConcurrent(t *testing.T) { // relevante com -race
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				textWidth("x", float64(10+j%5), 0)
			}
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestTextWidth|TestLoadFont|TestFont' -count=1`
Expected: FAIL (assinaturas antigas; `loadFont`/`fontExists` não existem).

- [ ] **Step 3: Reescrever font.go**

```go
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
	fontMu       sync.Mutex
	builtinOnce  sync.Once
	builtinFont  *text.GoTextFaceSource
	loadedFonts  = map[int64]*text.GoTextFaceSource{}
	nextFontID   int64
	fontFaces    = map[faceKey]*text.GoTextFace{}
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
```

- [ ] **Step 4: Ajustar a chamada no render**

`render.go`, caso `cmdText` — o id vem do comando na Task 2; por enquanto
passar 0 para compilar:

```go
		drawText(screen, c.text, c.size, 0, op)
```

- [ ] **Step 5: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add font.go font_test.go render.go
git commit -m "feat(fonte): registro de fontes com cache por par (fonte, tamanho)"
```

---

### Task 2: Comando text com id de fonte e exports

**Files:**
- Modify: `commands.go` (arity `text` = 10, campo `font`)
- Modify: `engine.go` (validação de fonte no `handleFlip`)
- Modify: `render.go` (passar `c.font`)
- Modify: `main.go` (`game_load_font`, `game_text_width` com 3 params)
- Modify: `noxy_ext.toml`
- Test: `commands_test.go`, `engine_test.go`

**Interfaces:**
- Consumes: `loadFont`, `fontExists`, `textWidth(s, size, fontID)` da Task 1.
- Produces: comando `["text", s, x, y, size, r, g, b, a, font_id]` (aridade 10) e campo `command.font int64`; exports `game_load_font(string) -> map[string]any` (`{"id"}`) e `game_text_width(string, int, int) -> float`. O wrapper (Task 7) chama esses nomes.

- [ ] **Step 1: Escrever os testes**

Em `commands_test.go`: atualizar os casos de `text` existentes para aridade
10 (o `TestDecodeFrameAllTags` ganha `int64(0)` no fim do comando `text`, e
o caso `"text type"` de `TestDecodeFrameErrors` também), e acrescentar:

```go
func TestDecodeTextFont(t *testing.T) {
	cmds, err := decodeFrame([]any{[]any{"text", "hi", 1.0, 2.0, int64(16), int64(1), int64(2), int64(3), int64(255), int64(7)}})
	if err != nil {
		t.Fatal(err)
	}
	if cmds[0].font != 7 {
		t.Fatalf("want font 7, got %+v", cmds[0])
	}
}

func TestDecodeTextFontType(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"text", "hi", 1.0, 2.0, int64(16), int64(1), int64(2), int64(3), int64(255), "x"}})
	if err == nil || !strings.Contains(err.Error(), `element 9 of "text": expected int, got string`) {
		t.Fatalf("want font type error, got %v", err)
	}
}
```

Em `engine_test.go`:

```go
func TestFlipRejectsUnknownFont(t *testing.T) {
	e := startedEngine(t)
	frame := []any{[]any{"text", "hi", 0.0, 0.0, int64(16), int64(0), int64(0), int64(0), int64(255), int64(4242)}}
	_, err := e.handleFlip(context.Background(), frame)
	if err == nil || !strings.Contains(err.Error(), "command 0: unknown font 4242 (not returned by load_font)") {
		t.Fatalf("want unknown font error, got %v", err)
	}
	if e.hasPendingFrame() {
		t.Fatal("frame inválido não deve ficar pendente")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestDecodeText|TestFlipRejectsUnknownFont' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar**

`commands.go`:
- `arity["text"] = 10`.
- `command` ganha `font int64` (junto de `image`); comentário do struct:
  `text: text x y size color font`.
- decode do `text`:

```go
	case "text":
		c.kind = cmdText
		c.text = r.str(1)
		c.x, c.y, c.size = r.num(2), r.num(3), r.num(4)
		c.color = r.color(5)
		c.font = r.integer(9)
```

`engine.go`, no loop de validação do `handleFlip` — acrescentar antes do
`if c.kind != cmdImage { continue }`:

```go
		if c.kind == cmdText {
			if !fontExists(c.font) {
				e.mu.Unlock()
				return nil, fmt.Errorf("command %d: unknown font %d (not returned by load_font)", i, c.font)
			}
			continue
		}
```

**Atenção:** `fontExists` pega `fontMu`, e aqui `e.mu` já está travado. Não
há inversão de ordem (nenhum caminho pega `e.mu` com `fontMu` travado), mas
para manter simples a ordem é sempre `e.mu` → `fontMu`.

`render.go`, caso `cmdText`: trocar o `0` pelo id do comando:

```go
		drawText(screen, c.text, c.size, c.font, op)
```

`main.go`: trocar o handler de largura e acrescentar o de fonte:

```go
	p.Handle("game_text_width", noxyplugin.Func3(handleTextWidth))
	p.Handle("game_load_font", noxyplugin.Func1(handleLoadFont))
```

e as funções (substituindo o `handleTextWidth` atual):

```go
// handleTextWidth: game_text_width(s, size, font_id) -> float. Não exige
// game_init.
func handleTextWidth(ctx context.Context, s string, size, fontID int64) (float64, error) {
	if size <= 0 {
		return 0, fmt.Errorf("size must be positive, got %d", size)
	}
	return textWidth(s, float64(size), fontID)
}

// handleLoadFont: game_load_font(path) -> {"id"}. Aceita TTF e OTF.
func handleLoadFont(ctx context.Context, path string) (map[string]any, error) {
	id, err := loadFont(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}
```

`noxy_ext.toml`: trocar o export de largura e acrescentar o de fonte:

```toml
[[export]]
name = "game_text_width"        # (s, size, font_id) -> largura em px
params = ["string", "int", "int"]
returns = "float"

[[export]]
name = "game_load_font"         # (path) -> {"id"}; TTF ou OTF
params = ["string"]
returns = "map[string]any"
stateful = true
```

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add commands.go engine.go render.go main.go noxy_ext.toml commands_test.go engine_test.go
git commit -m "feat(fonte): comando text com id de fonte e export game_load_font"
```

---

### Task 3: Janela — fullscreen e FPS

**Files:**
- Modify: `engine.go` (interface `platform`, pedidos pendentes, `tick`)
- Modify: `render.go` (`ebitenInput` implementa `platform`)
- Modify: `main.go` (handlers)
- Modify: `noxy_ext.toml`
- Test: `engine_test.go`

**Interfaces:**
- Consumes: nada das tasks anteriores.
- Produces: interface `platform` (estende `inputReader` com `setFullscreen(bool)` e `setTPS(int)`); `tick(p platform) bool`; handlers `handleSetFullscreen`, `handleSetFps`; exports `game_set_fullscreen(bool) -> void` e `game_set_fps(int) -> void`. As Tasks 4 e 5 estendem a mesma interface `platform`.

- [ ] **Step 1: Escrever os testes**

Em `engine_test.go`, estender a `fakeInput` (ela passa a ser a `platform`
dos testes) e acrescentar os casos:

```go
// campos novos em fakeInput (struct existente):
//     fullscreen *bool
//     tps        *int
// e os métodos:

func (f *fakeInput) setFullscreen(on bool) { f.fullscreen = &on }
func (f *fakeInput) setTPS(n int)          { f.tps = &n }
```

**Atenção:** os métodos novos têm receptor **ponteiro** (precisam gravar), e
a `fakeInput` atual é usada por valor (`e.tick(fakeInput{...})`). Trocar
todos os usos existentes para `&fakeInput{...}` e os receptores dos métodos
antigos para ponteiro também, para a interface ser satisfeita por `*fakeInput`.

```go
func TestSetFullscreenAppliedOnTick(t *testing.T) {
	e := startedEngine(t)
	if _, err := e.handleSetFullscreen(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.handleSetFps(context.Background(), 30); err != nil {
		t.Fatal(err)
	}
	in := &fakeInput{}
	e.tick(in) // aplica mesmo sem frame pendente
	if in.fullscreen == nil || !*in.fullscreen {
		t.Fatalf("fullscreen não aplicado: %v", in.fullscreen)
	}
	if in.tps == nil || *in.tps != 30 {
		t.Fatalf("tps não aplicado: %v", in.tps)
	}
	in2 := &fakeInput{}
	e.tick(in2) // pedido consumido: não repete
	if in2.fullscreen != nil || in2.tps != nil {
		t.Fatal("pedido deve ser aplicado uma vez só")
	}
}

func TestWindowOpsRequireInit(t *testing.T) {
	e := newEngine()
	if _, err := e.handleSetFullscreen(context.Background(), true); !errors.Is(err, errNotInitialized) {
		t.Fatalf("fullscreen sem init: %v", err)
	}
	if _, err := e.handleSetFps(context.Background(), 60); !errors.Is(err, errNotInitialized) {
		t.Fatalf("fps sem init: %v", err)
	}
}

func TestSetFpsRejectsZero(t *testing.T) {
	e := startedEngine(t)
	_, err := e.handleSetFps(context.Background(), 0)
	if err == nil || !strings.Contains(err.Error(), "fps must be at least 1, got 0") {
		t.Fatalf("want fps error, got %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestSetFullscreen|TestWindowOps|TestSetFps' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar**

`engine.go`:
- interface nova, logo abaixo de `inputReader`:

```go
// platform é o que o tick usa a cada Update: o input do frame e as
// operações de janela, que precisam rodar na thread principal. render.go dá
// a implementação sobre o Ebiten, os testes dão uma fake.
type platform interface {
	inputReader
	setFullscreen(on bool)
	setTPS(n int)
}
```

- campos no `engine` (junto de `pending`):

```go
	pendingFullscreen *bool
	pendingTPS        *int
```

- handlers:

```go
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
```

- `tick`: trocar a assinatura para `func (e *engine) tick(p platform) (terminate bool)`,
  trocar os usos de `in` por `p`, e aplicar os pedidos **antes** do retorno
  antecipado de "sem frame". Dentro da seção com `e.mu` travado, logo após o
  bloco `if p.windowClosing() { e.closed = true }`:

```go
	fs, tps := e.pendingFullscreen, e.pendingTPS
	e.pendingFullscreen, e.pendingTPS = nil, nil
```

  e depois do `e.mu.Unlock()` (antes do `if frame == nil`):

```go
	if fs != nil {
		p.setFullscreen(*fs)
	}
	if tps != nil {
		p.setTPS(*tps)
	}
```

`render.go`: `ebitenInput` ganha os dois métodos:

```go
func (ebitenInput) setFullscreen(on bool) { ebiten.SetFullscreen(on) }
func (ebitenInput) setTPS(n int)          { ebiten.SetTPS(n) }
```

`main.go`:

```go
	p.Handle("game_set_fullscreen", noxyplugin.Func1(e.handleSetFullscreen))
	p.Handle("game_set_fps", noxyplugin.Func1(e.handleSetFps))
```

`noxy_ext.toml`:

```toml
[[export]]
name = "game_set_fullscreen"    # (on): janela cheia; o Layout escala e faz letterbox
params = ["bool"]
returns = "void"
stateful = true

[[export]]
name = "game_set_fps"           # (n >= 1): ticks por segundo (padrao 60)
params = ["int"]
returns = "void"
stateful = true
```

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine.go render.go main.go noxy_ext.toml engine_test.go
git commit -m "feat(janela): set_fullscreen e set_fps aplicados na thread principal"
```

---

### Task 4: Roda do mouse e texto digitado

**Files:**
- Modify: `engine.go` (`platform`, `inputSnapshot`, `toMap`, `tick`)
- Modify: `render.go` (leitura via Ebiten)
- Test: `engine_test.go`

**Interfaces:**
- Consumes: interface `platform` da Task 3.
- Produces: `platform` ganha `wheel() (x, y float64)` e `textInput() string`; `inputSnapshot` ganha `WheelX, WheelY float64` e `TextInput string`; o mapa do snapshot ganha as chaves `wheel_x`, `wheel_y` (float) e `text_input` (string). O wrapper (Task 7) lê essas chaves.

- [ ] **Step 1: Escrever o teste**

Em `engine_test.go`, acrescentar os campos `wheelX, wheelY float64` e
`chars string` à `fakeInput`, com os métodos:

```go
func (f *fakeInput) wheel() (float64, float64) { return f.wheelX, f.wheelY }
func (f *fakeInput) textInput() string         { return f.chars }
```

e o caso (o padrão do flip concorrente é o mesmo do
`TestFlipDeliversFrameAndSnapshot` existente):

```go
func TestSnapshotCarriesWheelAndText(t *testing.T) {
	e := startedEngine(t)
	type result struct {
		snap map[string]any
		err  error
	}
	res := make(chan result, 1)
	go func() {
		s, err := e.handleFlip(context.Background(), []any{})
		res <- result{s, err}
	}()
	waitPending(t, e)
	e.tick(&fakeInput{wheelX: -1.5, wheelY: 3, chars: "oi"})
	r := <-res
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.snap["wheel_x"] != -1.5 || r.snap["wheel_y"] != 3.0 {
		t.Fatalf("wheel: %v %v", r.snap["wheel_x"], r.snap["wheel_y"])
	}
	if r.snap["text_input"] != "oi" {
		t.Fatalf("text_input: %v", r.snap["text_input"])
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestSnapshotCarriesWheelAndText' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar**

`engine.go`:
- `platform` ganha:

```go
	wheel() (x, y float64)
	textInput() string
```

- `inputSnapshot` ganha `WheelX, WheelY float64` e `TextInput string`;
  `toMap` ganha:

```go
		"wheel_x":    s.WheelX,
		"wheel_y":    s.WheelY,
		"text_input": s.TextInput,
```

- `tick`, no preenchimento do snapshot (junto de `mx, my, mdown, mpressed := p.mouse()`):

```go
	wx, wy := p.wheel()
```

  e no literal `inputSnapshot{...}`: `WheelX: wx, WheelY: wy, TextInput: p.textInput(),`.

`render.go`:

```go
func (ebitenInput) wheel() (float64, float64) { return ebiten.Wheel() }

func (ebitenInput) textInput() string {
	return string(ebiten.AppendInputChars(nil))
}
```

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine.go render.go engine_test.go
git commit -m "feat(input): roda do mouse e texto digitado no snapshot"
```

---

### Task 5: Gamepad

**Files:**
- Create: `pads.go`, `pads_test.go`
- Modify: `engine.go` (`platform`, snapshot)
- Modify: `render.go` (fonte de gamepad sobre o Ebiten)
- Test: `pads_test.go`, `engine_test.go`

**Interfaces:**
- Consumes: `platform` das Tasks 3–4.
- Produces: `padSource` (interface) e `gatherPads(src padSource) (down, pressed []string, axes []float64)`; `platform` ganha `pads() (down, pressed []string, axes []float64)`; snapshot ganha `PadDown, PadPressed []string` e `PadAxes []float64` → chaves `pad_down`, `pad_pressed`, `pad_axes`. O wrapper (Task 7) lê essas chaves e monta nomes `"<pad>:<botão>"`.

- [ ] **Step 1: Escrever os testes**

`pads_test.go` (novo):

```go
// pads_test.go — a montagem dos nomes de gamepad, com uma fonte fake: não
// há controle no CI, então o que se testa é a tabela e o formato.
package main

import (
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// fakePads implementa padSource com dados fixos.
type fakePads struct {
	count   int
	down    map[int][]ebiten.StandardGamepadButton
	just    map[int][]ebiten.StandardGamepadButton
	axes    map[int]map[ebiten.StandardGamepadAxis]float64
	trigger map[int]map[ebiten.StandardGamepadButton]float64
}

func (f fakePads) padCount() int { return f.count }
func (f fakePads) pressedButtons(pad int) []ebiten.StandardGamepadButton {
	return f.down[pad]
}
func (f fakePads) justPressedButtons(pad int) []ebiten.StandardGamepadButton {
	return f.just[pad]
}
func (f fakePads) axisValue(pad int, a ebiten.StandardGamepadAxis) float64 {
	return f.axes[pad][a]
}
func (f fakePads) buttonValue(pad int, b ebiten.StandardGamepadButton) float64 {
	return f.trigger[pad][b]
}

func TestGatherPadsEmpty(t *testing.T) {
	down, pressed, axes := gatherPads(fakePads{})
	if len(down) != 0 || len(pressed) != 0 || len(axes) != 0 {
		t.Fatalf("sem controles: %v %v %v", down, pressed, axes)
	}
}

func TestGatherPadsNames(t *testing.T) {
	src := fakePads{
		count: 2,
		down: map[int][]ebiten.StandardGamepadButton{
			0: {ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonLeftTop},
			1: {ebiten.StandardGamepadButtonCenterRight},
		},
		just: map[int][]ebiten.StandardGamepadButton{
			1: {ebiten.StandardGamepadButtonFrontTopLeft},
		},
	}
	down, pressed, axes := gatherPads(src)
	wantDown := []string{"0:a", "0:up", "1:start"}
	if !reflect.DeepEqual(down, wantDown) {
		t.Fatalf("down: want %v, got %v", wantDown, down)
	}
	wantPressed := []string{"1:lb"}
	if !reflect.DeepEqual(pressed, wantPressed) {
		t.Fatalf("pressed: want %v, got %v", wantPressed, pressed)
	}
	if len(axes) != 12 { // 6 por controle
		t.Fatalf("axes: want 12 values, got %d", len(axes))
	}
}

func TestGatherPadsAxes(t *testing.T) {
	src := fakePads{
		count: 1,
		axes: map[int]map[ebiten.StandardGamepadAxis]float64{
			0: {
				ebiten.StandardGamepadAxisLeftStickHorizontal: -0.5,
				ebiten.StandardGamepadAxisRightStickVertical:  0.25,
			},
		},
		trigger: map[int]map[ebiten.StandardGamepadButton]float64{
			0: {ebiten.StandardGamepadButtonFrontBottomRight: 0.75},
		},
	}
	_, _, axes := gatherPads(src)
	want := []float64{-0.5, 0, 0, 0.25, 0, 0.75} // left_x left_y right_x right_y lt rt
	if !reflect.DeepEqual(axes, want) {
		t.Fatalf("want %v, got %v", want, axes)
	}
}

func TestPadAxisIndex(t *testing.T) {
	for i, name := range padAxisNames {
		if got := padAxisIndex(name); got != i {
			t.Fatalf("%s: want %d, got %d", name, i, got)
		}
	}
	if padAxisIndex("nope") != -1 {
		t.Fatal("eixo desconhecido deve dar -1")
	}
}
```

Em `engine_test.go`, a `fakeInput` ganha:

```go
	padDown, padPressed []string
	padAxes             []float64
```

```go
func (f *fakeInput) pads() ([]string, []string, []float64) {
	return f.padDown, f.padPressed, f.padAxes
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestGatherPads|TestPadAxis' -count=1`
Expected: FAIL (pads.go não existe).

- [ ] **Step 3: Implementar pads.go**

```go
// pads.go — gamepads no layout padrão do Ebiten, traduzidos para o
// vocabulário do wrapper. A numeração dos controles é a posição na lista de
// conectados (o primeiro é sempre 0), não o id bruto do Ebiten.
//
// A leitura real fica atrás de padSource para o teste rodar sem hardware.
package main

import (
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

// padSource é de onde gatherPads lê; render.go dá a implementação sobre o
// Ebiten, os testes dão uma fake.
type padSource interface {
	padCount() int
	pressedButtons(pad int) []ebiten.StandardGamepadButton
	justPressedButtons(pad int) []ebiten.StandardGamepadButton
	axisValue(pad int, a ebiten.StandardGamepadAxis) float64
	buttonValue(pad int, b ebiten.StandardGamepadButton) float64
}

// padButtonNames traduz o layout padrão (web gamepad) para os nomes do
// wrapper.
var padButtonNames = map[ebiten.StandardGamepadButton]string{
	ebiten.StandardGamepadButtonRightBottom:     "a",
	ebiten.StandardGamepadButtonRightRight:      "b",
	ebiten.StandardGamepadButtonRightLeft:       "x",
	ebiten.StandardGamepadButtonRightTop:        "y",
	ebiten.StandardGamepadButtonLeftTop:         "up",
	ebiten.StandardGamepadButtonLeftBottom:      "down",
	ebiten.StandardGamepadButtonLeftLeft:        "left",
	ebiten.StandardGamepadButtonLeftRight:       "right",
	ebiten.StandardGamepadButtonCenterLeft:      "back",
	ebiten.StandardGamepadButtonCenterRight:     "start",
	ebiten.StandardGamepadButtonCenterCenter:    "guide",
	ebiten.StandardGamepadButtonFrontTopLeft:    "lb",
	ebiten.StandardGamepadButtonFrontTopRight:   "rb",
	ebiten.StandardGamepadButtonFrontBottomLeft: "lt",
	ebiten.StandardGamepadButtonFrontBottomRight: "rt",
	ebiten.StandardGamepadButtonLeftStick:       "lstick",
	ebiten.StandardGamepadButtonRightStick:      "rstick",
}

// padAxisNames é a ordem dos eixos em pad_axes (6 por controle).
var padAxisNames = []string{"left_x", "left_y", "right_x", "right_y", "lt", "rt"}

// padAxisIndex é a posição do eixo dentro do bloco de um controle, ou -1.
func padAxisIndex(name string) int {
	for i, n := range padAxisNames {
		if n == name {
			return i
		}
	}
	return -1
}

var padStickAxes = []ebiten.StandardGamepadAxis{
	ebiten.StandardGamepadAxisLeftStickHorizontal,
	ebiten.StandardGamepadAxisLeftStickVertical,
	ebiten.StandardGamepadAxisRightStickHorizontal,
	ebiten.StandardGamepadAxisRightStickVertical,
}

// gatherPads monta os arrays do snapshot: nomes prefixados pelo número do
// controle ("0:a") e os 6 eixos de cada um, em sequência.
func gatherPads(src padSource) (down, pressed []string, axes []float64) {
	n := src.padCount()
	down, pressed, axes = []string{}, []string{}, []float64{}
	for pad := 0; pad < n; pad++ {
		prefix := strconv.Itoa(pad) + ":"
		for _, b := range src.pressedButtons(pad) {
			if name, ok := padButtonNames[b]; ok {
				down = append(down, prefix+name)
			}
		}
		for _, b := range src.justPressedButtons(pad) {
			if name, ok := padButtonNames[b]; ok {
				pressed = append(pressed, prefix+name)
			}
		}
		for _, a := range padStickAxes {
			axes = append(axes, src.axisValue(pad, a))
		}
		axes = append(axes,
			src.buttonValue(pad, ebiten.StandardGamepadButtonFrontBottomLeft),
			src.buttonValue(pad, ebiten.StandardGamepadButtonFrontBottomRight))
	}
	return down, pressed, axes
}
```

`render.go`: a fonte real e o método do `platform`:

```go
// ebitenPads lê os controles com layout padrão; a posição na lista é o
// número do controle visto pelo script.
type ebitenPads struct{ ids []ebiten.GamepadID }

func connectedPads() ebitenPads {
	var out []ebiten.GamepadID
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			out = append(out, id)
		}
	}
	return ebitenPads{ids: out}
}

func (p ebitenPads) padCount() int { return len(p.ids) }

func (p ebitenPads) pressedButtons(pad int) []ebiten.StandardGamepadButton {
	return inpututil.AppendPressedStandardGamepadButtons(p.ids[pad], nil)
}

func (p ebitenPads) justPressedButtons(pad int) []ebiten.StandardGamepadButton {
	return inpututil.AppendJustPressedStandardGamepadButtons(p.ids[pad], nil)
}

func (p ebitenPads) axisValue(pad int, a ebiten.StandardGamepadAxis) float64 {
	return ebiten.StandardGamepadAxisValue(p.ids[pad], a)
}

func (p ebitenPads) buttonValue(pad int, b ebiten.StandardGamepadButton) float64 {
	return ebiten.StandardGamepadButtonValue(p.ids[pad], b)
}

func (ebitenInput) pads() ([]string, []string, []float64) {
	return gatherPads(connectedPads())
}
```

`engine.go`:
- `platform` ganha `pads() (down, pressed []string, axes []float64)`.
- `inputSnapshot` ganha `PadDown, PadPressed []string` e `PadAxes []float64`.
- `toMap` ganha:

```go
		"pad_down":    nonNil(s.PadDown),
		"pad_pressed": nonNil(s.PadPressed),
		"pad_axes":    nonNilF(s.PadAxes),
```

- helper novo ao lado de `nonNil`:

```go
func nonNilF(xs []float64) []float64 {
	if xs == nil {
		return []float64{}
	}
	return xs
}
```

- `tick`, junto das outras leituras: `pdown, ppressed, paxes := p.pads()` e
  no literal do snapshot: `PadDown: pdown, PadPressed: ppressed, PadAxes: paxes,`.

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pads.go pads_test.go engine.go render.go engine_test.go
git commit -m "feat(input): gamepad no layout padrao (botoes, eixos, varios controles)"
```

---

### Task 6: Polígonos

**Files:**
- Modify: `commands.go` (tag `polygon`, leitor de pontos)
- Modify: `render.go` (vector.Path)
- Test: `commands_test.go`

**Interfaces:**
- Consumes: `color4` e o `reader` existentes.
- Produces: comando `["polygon", [x1, y1, x2, y2, ...], r, g, b, a, thickness]` (aridade 7), `cmdPolygon` e `command.points []float64`. O wrapper (Task 7) manda `draw_polygon`/`draw_polygon_outline`.

- [ ] **Step 1: Escrever os testes**

Em `commands_test.go`:

```go
func TestDecodePolygon(t *testing.T) {
	cmds, err := decodeFrame([]any{
		[]any{"polygon", []any{0.0, 0.0, 10.0, 0.0, 5.0, 8.0}, int64(1), int64(2), int64(3), int64(255), 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := cmds[0]
	if c.kind != cmdPolygon || c.thickness != 0 || c.color != (color4{1, 2, 3, 255}) {
		t.Fatalf("polygon: %+v", c)
	}
	want := []float64{0, 0, 10, 0, 5, 8}
	if !reflect.DeepEqual(c.points, want) {
		t.Fatalf("points: want %v, got %v", want, c.points)
	}
}

func TestDecodePolygonErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  []any
		want string
	}{
		{"not an array", []any{[]any{"polygon", 1.0, int64(0), int64(0), int64(0), int64(255), 0}},
			`element 1 of "polygon": expected array of numbers, got float`},
		{"odd count", []any{[]any{"polygon", []any{0.0, 0.0, 1.0, 1.0, 2.0}, int64(0), int64(0), int64(0), int64(255), 0}},
			`element 1 of "polygon": point list must have an even number of values, got 5`},
		{"too few points", []any{[]any{"polygon", []any{0.0, 0.0, 1.0, 1.0}, int64(0), int64(0), int64(0), int64(255), 0}},
			`element 1 of "polygon": needs at least 3 points, got 2`},
		{"bad value", []any{[]any{"polygon", []any{0.0, 0.0, 1.0, 1.0, "x", 2.0}, int64(0), int64(0), int64(0), int64(255), 0}},
			`element 1 of "polygon": expected array of numbers, got string`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeFrame(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}
```

(o arquivo passa a importar `reflect`.)

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestDecodePolygon' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar**

`commands.go`:
- `cmdPolygon` no fim do bloco `const` de `cmdKind`.
- `arity["polygon"] = 7`.
- `command` ganha `points []float64`; comentário do struct:
  `polygon: points color thickness`.
- decode:

```go
	case "polygon":
		c.kind = cmdPolygon
		c.points = r.points(1)
		c.color = r.color(2)
		c.thickness = r.thickness(6)
```

- leitor novo:

```go
// points lê a lista plana [x1, y1, x2, y2, ...] do elemento i.
func (r *reader) points(i int) []float64 {
	raw, ok := r.parts[i].([]any)
	if !ok {
		r.fail(i, "expected array of numbers, got %s", typeName(r.parts[i]))
		return nil
	}
	out := make([]float64, 0, len(raw))
	for _, v := range raw {
		switch n := v.(type) {
		case int64:
			out = append(out, float64(n))
		case int:
			out = append(out, float64(n))
		case float64:
			out = append(out, n)
		default:
			r.fail(i, "expected array of numbers, got %s", typeName(v))
			return nil
		}
	}
	if len(out)%2 != 0 {
		r.fail(i, "point list must have an even number of values, got %d", len(out))
		return nil
	}
	if len(out)/2 < 3 {
		r.fail(i, "needs at least 3 points, got %d", len(out)/2)
		return nil
	}
	return out
}
```

`render.go`, caso novo no `switch c.kind` (import novo: `"github.com/hajimehoshi/ebiten/v2/vector"` já está lá):

```go
	case cmdPolygon:
		var path vector.Path
		path.MoveTo(float32(c.points[0]), float32(c.points[1]))
		for i := 2; i < len(c.points); i += 2 {
			path.LineTo(float32(c.points[i]), float32(c.points[i+1]))
		}
		path.Close()
		draw := &vector.DrawPathOptions{AntiAlias: true}
		draw.ColorScale.ScaleWithColor(clr)
		if c.thickness == 0 {
			vector.FillPath(screen, &path, &vector.FillOptions{}, draw)
		} else {
			vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: float32(c.thickness)}, draw)
		}
```

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add commands.go render.go commands_test.go
git commit -m "feat(desenho): poligono preenchido e contornado"
```

---

### Task 7: Wrapper v0.3 e validação local

**Files:**
- Modify: `noxy_game_engine.nx`

**Interfaces:**
- Consumes: os exports das Tasks 2, 3 e as chaves de snapshot das Tasks 4–5, e o comando `polygon` da Task 6.
- Produces: API pública v0.3 — `Font`, `load_font`, `set_font`, `reset_font`, `set_fullscreen`, `set_fps`, `wheel_x`, `wheel_y`, `text_input`, `gamepad_count`, `gamepad_down`, `gamepad_pressed`, `gamepad_axis`, `draw_polygon`, `draw_polygon_outline`, `camera_set`, `camera_reset`, `camera_pos`, `rect_overlaps`, `circle_overlaps`, `circle_rect_overlaps`, `point_in_rect`. O exemplo da Task 8 usa esses nomes.

- [ ] **Step 1: Editar noxy_game_engine.nx**

Struct e estado novos (junto de `Sound`, e do bloco de estado de módulo):

```noxy
struct Font
    id: int
end
```

```noxy
let _font: int = 0             // 0 = fonte embutida
let _cam_x: float = 0.0
let _cam_y: float = 0.0
let _wheel_x: float = 0.0
let _wheel_y: float = 0.0
let _text_input: string = ""
let _pad_down: string[] = []
let _pad_pressed: string[] = []
let _pad_axes: float[] = []
```

`flip` passa a ler as chaves novas (acrescentar antes do fim da função):

```noxy
    let wx: float = snap["wheel_x"]
    _wheel_x = wx
    let wy: float = snap["wheel_y"]
    _wheel_y = wy
    let ti: string = snap["text_input"]
    _text_input = ti
    let pd: string[] = snap["pad_down"]
    _pad_down = pd
    let pp: string[] = snap["pad_pressed"]
    _pad_pressed = pp
    let pa: float[] = snap["pad_axes"]
    _pad_axes = pa
```

Câmera aplicada nos desenhos — cada `append` de posição subtrai o offset.
As funções ficam assim (substituem as atuais):

```noxy
func draw_rect(x: float, y: float, w: float, h: float, c: Color) -> void
    append(ref _cmds, ["rect", x - _cam_x, y - _cam_y, w, h, c.r, c.g, c.b, c.a, 0])
end

func draw_rect_outline(x: float, y: float, w: float, h: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["rect", x - _cam_x, y - _cam_y, w, h, c.r, c.g, c.b, c.a, thickness])
end

func draw_circle(cx: float, cy: float, radius: float, c: Color) -> void
    append(ref _cmds, ["circle", cx - _cam_x, cy - _cam_y, radius, c.r, c.g, c.b, c.a, 0])
end

func draw_circle_outline(cx: float, cy: float, radius: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["circle", cx - _cam_x, cy - _cam_y, radius, c.r, c.g, c.b, c.a, thickness])
end

func draw_line(x1: float, y1: float, x2: float, y2: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["line", x1 - _cam_x, y1 - _cam_y, x2 - _cam_x, y2 - _cam_y, c.r, c.g, c.b, c.a, thickness])
end

func draw_text(s: string, x: float, y: float, size: int, c: Color) -> void
    append(ref _cmds, ["text", s, x - _cam_x, y - _cam_y, size, c.r, c.g, c.b, c.a, _font])
end

func draw_image(img: Image, x: float, y: float) -> void
    append(ref _cmds, ["image", img.id, 0, 0, img.width, img.height, x - _cam_x, y - _cam_y, 1.0, 0.0, 1.0, false])
end

func draw_image_ex(img: Image, x: float, y: float, scale: float, angle_deg: float, opacity: float, flip_x: bool) -> void
    append(ref _cmds, ["image", img.id, 0, 0, img.width, img.height, x - _cam_x, y - _cam_y, scale, angle_deg, opacity, flip_x])
end

func draw_image_sub(img: Image, sx: float, sy: float, sw: float, sh: float, x: float, y: float) -> void
    append(ref _cmds, ["image", img.id, sx, sy, sw, sh, x - _cam_x, y - _cam_y, 1.0, 0.0, 1.0, false])
end

func draw_image_sub_ex(img: Image, sx: float, sy: float, sw: float, sh: float, x: float, y: float, scale: float, angle_deg: float, opacity: float, flip_x: bool) -> void
    append(ref _cmds, ["image", img.id, sx, sy, sw, sh, x - _cam_x, y - _cam_y, scale, angle_deg, opacity, flip_x])
end
```

Polígono (a câmera é aplicada ponto a ponto, então os pontos são copiados):

```noxy
// draw_polygon fills the polygon through the points [x1, y1, x2, y2, ...]
// (at least 3 points).
func draw_polygon(points: float[], c: Color) -> void
    append(ref _cmds, ["polygon", _shift(points), c.r, c.g, c.b, c.a, 0])
end

// draw_polygon_outline draws only the outline, `thickness` px wide.
func draw_polygon_outline(points: float[], c: Color, thickness: float) -> void
    append(ref _cmds, ["polygon", _shift(points), c.r, c.g, c.b, c.a, thickness])
end

// _shift applies the camera offset to a flat point list.
func _shift(points: float[]) -> float[]
    let out: float[] = []
    let i = 0
    while i < length(points) do
        if i % 2 == 0 then
            append(ref out, points[i] - _cam_x)
        else
            append(ref out, points[i] - _cam_y)
        end
        i = i + 1
    end
    return out
end
```

Fonte, janela, input novo, câmera e colisão (seções novas):

```noxy
// ---- fonts ----

// load_font registers a TTF or OTF file. Raises if it is missing or is not
// a font.
func load_font(path: string) -> Font
    let info: map[string, any] = game_load_font(path)
    let id: int = info["id"]
    return Font(id)
end

// set_font makes draw_text and text_width use `f` from here on.
func set_font(f: Font) -> void
    _font = f.id
end

// reset_font goes back to the built-in Go Regular font.
func reset_font() -> void
    _font = 0
end

// ---- window ----

// set_fullscreen switches the window to fullscreen and back; the drawing
// area keeps its logical size and is scaled (with letterboxing).
func set_fullscreen(on: bool) -> void
    game_set_fullscreen(on)
end

// set_fps sets how many times per second flip returns (default 60).
func set_fps(n: int) -> void
    game_set_fps(n)
end

// ---- extra input ----

// wheel_x / wheel_y: how much the mouse wheel moved during the last frame
// (0 when it did not move).
func wheel_x() -> float
    return _wheel_x
end

func wheel_y() -> float
    return _wheel_y
end

// text_input: the characters typed during the last frame, for name entry
// and the like. Empty when nothing was typed.
func text_input() -> string
    return _text_input
end

// gamepad_count: how many controllers with a standard layout are connected.
// Pad numbers go from 0 to gamepad_count() - 1.
func gamepad_count() -> int
    return length(_pad_axes) / 6
end

// gamepad_down: the button is held. Names: "a" "b" "x" "y", "up" "down"
// "left" "right", "start" "back" "guide", "lb" "rb" "lt" "rt", "lstick"
// "rstick".
func gamepad_down(pad: int, button: string) -> bool
    return _has(_pad_down, f"{pad}:{button}")
end

// gamepad_pressed: the button went down during the last frame.
func gamepad_pressed(pad: int, button: string) -> bool
    return _has(_pad_pressed, f"{pad}:{button}")
end

// gamepad_axis: "left_x" "left_y" "right_x" "right_y" (-1..1) or "lt" "rt"
// (0..1). Unknown axis or missing controller gives 0.0.
func gamepad_axis(pad: int, axis: string) -> float
    let idx = _axis_index(axis)
    if idx < 0 then
        return 0.0
    end
    let at = pad * 6 + idx
    if pad < 0 || at >= length(_pad_axes) then
        return 0.0
    end
    return _pad_axes[at]
end

func _axis_index(axis: string) -> int
    if axis == "left_x" then return 0 end
    if axis == "left_y" then return 1 end
    if axis == "right_x" then return 2 end
    if axis == "right_y" then return 3 end
    if axis == "lt" then return 4 end
    if axis == "rt" then return 5 end
    return -1
end

// ---- camera ----

// camera_set shifts every draw call by (-x, -y): pass the world position
// you want at the top-left corner. clear() is not affected. Draw the world,
// call camera_reset(), then draw the UI.
func camera_set(x: float, y: float) -> void
    _cam_x = x
    _cam_y = y
end

func camera_reset() -> void
    _cam_x = 0.0
    _cam_y = 0.0
end

func camera_pos() -> Point
    return Point(to_int(_cam_x), to_int(_cam_y))
end

// ---- collision ----

// rect_overlaps: do the two rectangles (x, y, w, h) overlap?
func rect_overlaps(ax: float, ay: float, aw: float, ah: float, bx: float, by: float, bw: float, bh: float) -> bool
    return ax < bx + bw && ax + aw > bx && ay < by + bh && ay + ah > by
end

// circle_overlaps: do the two circles (x, y, radius) overlap?
func circle_overlaps(ax: float, ay: float, ar: float, bx: float, by: float, br: float) -> bool
    let dx = ax - bx
    let dy = ay - by
    let r = ar + br
    return dx * dx + dy * dy < r * r
end

// circle_rect_overlaps: does the circle (cx, cy, r) touch the rectangle?
func circle_rect_overlaps(cx: float, cy: float, r: float, x: float, y: float, w: float, h: float) -> bool
    let nx = cx
    if nx < x then nx = x end
    if nx > x + w then nx = x + w end
    let ny = cy
    if ny < y then ny = y end
    if ny > y + h then ny = y + h end
    let dx = cx - nx
    let dy = cy - ny
    return dx * dx + dy * dy < r * r
end

// point_in_rect: is (px, py) inside the rectangle?
func point_in_rect(px: float, py: float, x: float, y: float, w: float, h: float) -> bool
    return px >= x && px <= x + w && py >= y && py <= y + h
end
```

`text_width` passa o id da fonte corrente:

```noxy
func text_width(s: string, size: int) -> float
    let w: float = game_text_width(s, size, _font)
    return w
end
```

Atualizar o comentário do topo do arquivo com os natives novos
(`game_load_font`, `game_set_fullscreen`, `game_set_fps`).

- [ ] **Step 2: Rebuild e smoke**

Run (Bash):
```bash
cd /c/Users/sandr/Documents/noxy_game_engine && go build -o bin/noxy-plugin-game-windows-amd64.exe . && cd /c/Users/sandr/Documents/noxy && ./noxy.exe ../noxy_game_engine/examples/smoke.nx
```
Expected: janela por ~0,5 s e `ok`.

- [ ] **Step 3: Script descartável cobrindo o v0.3**

Criar `<scratchpad>/v03_check.nx` (rodar com cwd em
`C:\Users\sandr\Documents\noxy`), incluindo a fonte do sistema para validar
`load_font` sem commitar asset:

```noxy
use github_com.estevaofon.noxy_game_engine as game

game.init(320, 240, "v0.3 check")
game.set_fps(60)

let f: game.Font = game.load_font("C:/Windows/Fonts/arial.ttf")
game.set_font(f)
let w1: float = game.text_width("Game over", 30)
game.reset_font()
let w2: float = game.text_width("Game over", 30)
print(f"arial={w1} builtin={w2}")

print(f"pads={game.gamepad_count()}")
print(f"overlap={game.rect_overlaps(0.0, 0.0, 10.0, 10.0, 5.0, 5.0, 10.0, 10.0)}")
print(f"circle={game.circle_overlaps(0.0, 0.0, 5.0, 3.0, 0.0, 5.0)}")
print(f"cr={game.circle_rect_overlaps(0.0, 0.0, 5.0, 3.0, 3.0, 10.0, 10.0)}")
print(f"pt={game.point_in_rect(5.0, 5.0, 0.0, 0.0, 10.0, 10.0)}")

let tri: float[] = [160.0, 40.0, 220.0, 140.0, 100.0, 140.0]
let i = 0
while game.running() && i < 90 do
    game.camera_set(to_float(i), 0.0)
    game.clear(game.rgb(20, 20, 40))
    game.draw_polygon(tri, game.rgb(200, 80, 60))
    game.draw_polygon_outline(tri, game.WHITE, 2.0)
    game.camera_reset()
    game.set_font(f)
    game.draw_text("arial", 10.0, 10.0, 20, game.WHITE)
    game.reset_font()
    game.draw_text(f"wheel {game.wheel_y()} txt '{game.text_input()}'", 10.0, 210.0, 14, game.WHITE)
    game.flip()
    i = i + 1
end
game.quit()
print("v0.3 ok")
```

Run: `cd /c/Users/sandr/Documents/noxy && ./noxy.exe <scratchpad>/v03_check.nx`
Expected: imprime as larguras (arial ≠ builtin), `pads=0`, as quatro
colisões `true`, e termina com `v0.3 ok`, exit 0.

- [ ] **Step 4: Commit**

```bash
git add noxy_game_engine.nx
git commit -m "feat(wrapper): API v0.3 — fonte, janela, input novo, poligono, camera e colisao"
```

---

### Task 8: Exemplo showcase, README, issue e push

**Files:**
- Create: `examples/showcase.nx`
- Modify: `README.md`
- Modify: issue #1 (via `gh`)

**Interfaces:**
- Consumes: a API v0.3 do wrapper (Task 7).
- Produces: docs publicadas; branch pronta para a tag v0.3.0 (que só sai com confirmação do usuário).

- [ ] **Step 1: Criar examples/showcase.nx**

```noxy
// showcase.nx — the v0.3 additions in one screen: a polygon world you can
// pan with the arrow keys (camera), zoom-free scroll feedback from the
// mouse wheel, a text field fed by text_input(), F for fullscreen and
// collision helpers lighting up a box under the cursor.
// Run from the repository root: noxy examples/showcase.nx
use github_com.estevaofon.noxy_game_engine as game

let W = 640.0
let H = 400.0

game.init(640, 400, "Showcase")

let cam_x = 0.0
let cam_y = 0.0
let name = ""
let wheel_total = 0.0
let full = false

// a few polygons scattered in world space
let tri: float[] = [100.0, 100.0, 180.0, 100.0, 140.0, 40.0]
let diamond: float[] = [320.0, 60.0, 380.0, 120.0, 320.0, 180.0, 260.0, 120.0]
let box_x = 420.0
let box_y = 80.0
let box_w = 120.0
let box_h = 90.0

while game.running() do
    if game.key_pressed("escape") then
        game.stop()
    end
    if game.key_pressed("f") then
        full = !full
        game.set_fullscreen(full)
    end
    let dt: float = game.delta()
    let speed = 200.0 * dt
    if game.key_down("left") then cam_x = cam_x - speed end
    if game.key_down("right") then cam_x = cam_x + speed end
    if game.key_down("up") then cam_y = cam_y - speed end
    if game.key_down("down") then cam_y = cam_y + speed end
    wheel_total = wheel_total + game.wheel_y()
    name = name + game.text_input()
    if game.key_pressed("backspace") && length(name) > 0 then
        name = substring(name, 0, length(name) - 1)
    end

    // mouse in world coordinates: screen position plus the camera offset
    let m: game.Point = game.mouse_pos()
    let mx = to_float(m.x) + cam_x
    let my = to_float(m.y) + cam_y
    let hot = game.point_in_rect(mx, my, box_x, box_y, box_w, box_h)

    game.clear(game.rgb(24, 26, 34))
    game.camera_set(cam_x, cam_y)
    game.draw_polygon(tri, game.rgb(220, 90, 70))
    game.draw_polygon_outline(diamond, game.rgb(120, 200, 255), 3.0)
    let box_color: game.Color = game.rgba(90, 200, 120, 120)
    if hot then
        box_color = game.rgba(90, 200, 120, 220)
    end
    game.draw_rect(box_x, box_y, box_w, box_h, box_color)
    game.draw_text("world space", box_x + 8.0, box_y + 8.0, 14, game.WHITE)
    game.camera_reset()

    game.draw_rect(0.0, H - 90.0, W, 90.0, game.rgba(0, 0, 0, 160))
    game.draw_text("arrows pan · wheel scrolls · type a name · F fullscreen · Escape quits", 12.0, H - 80.0, 14, game.WHITE)
    game.draw_text(f"camera {to_int(cam_x)},{to_int(cam_y)}   wheel {wheel_total}   pads {game.gamepad_count()}", 12.0, H - 58.0, 14, game.rgb(180, 180, 200))
    game.draw_text(f"name: {name}_", 12.0, H - 34.0, 16, game.YELLOW)
    game.flip()
end
game.quit()
```

**Atenção:** se `substring` não existir na stdlib do Noxy com essa
assinatura, conferir o nome real antes (`grep -rn "func substring"
/c/Users/sandr/Documents/noxy/stdlib`) e ajustar; se não houver equivalente,
trocar o backspace por "limpa o nome inteiro" (`name = ""`).

- [ ] **Step 2: Rodar o showcase**

Run (cópia no scratchpad com caminho e limite de frames, como nas
validações anteriores):
```bash
SP=<scratchpad>
sed -e 's|^while game.running() do|let frames = 0\nwhile game.running() \&\& frames < 240 do\n    frames = frames + 1|' examples/showcase.nx > "$SP/showcase_auto.nx"
cd /c/Users/sandr/Documents/noxy && ./noxy.exe "$SP/showcase_auto.nx"
```
Expected: exit 0, sem erro de runtime.

- [ ] **Step 3: Atualizar o README**

Mudanças, seção a seção:
- Instalação: exemplo vira `@v0.3.0`.
- Tabela de desenho: linhas novas para `draw_polygon(points: float[], c)` e
  `draw_polygon_outline(points, c, thickness)` (pontos planos `[x1, y1, x2,
  y2, ...]`, mínimo 3 pontos).
- Seção **Fonts** nova (logo após Drawing): `load_font(path) -> Font`,
  `set_font(f)`, `reset_font()`; nota de que `draw_text`/`text_width` usam a
  fonte corrente, que TTF e OTF servem, e que `load_font` não exige `init`.
- Seção **Window** nova: `set_fullscreen(on)` (o desenho mantém o tamanho
  lógico e é escalado com letterbox) e `set_fps(n)` (padrão 60; `flip`
  passa a devolver n vezes por segundo).
- Seção Input: `wheel_x()`/`wheel_y()`, `text_input()`, e a subseção de
  gamepad com `gamepad_count()`, `gamepad_down(pad, button)`,
  `gamepad_pressed(pad, button)`, `gamepad_axis(pad, axis)`, a tabela de
  nomes de botão/eixo, a numeração por posição e a nota de que controles sem
  layout padrão são ignorados.
- Seção **Camera and collision** nova: `camera_set(x, y)`,
  `camera_reset()`, `camera_pos()`, o padrão mundo→UI, o fato de `clear` não
  ser afetado e de `mouse_pos()` ser em coordenadas de tela; e
  `rect_overlaps`, `circle_overlaps`, `circle_rect_overlaps`,
  `point_in_rect`. Nota de que tudo isso é Noxy puro no wrapper, sem
  chamada ao processo.
- Lista de exemplos: acrescentar `showcase.nx`.

- [ ] **Step 4: Verificação final e push**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS. Conferir `git status` (nada de binário ou arquivo do
scratchpad no stage).

```bash
git add examples/showcase.nx README.md
git commit -m "docs(readme,examples): showcase do v0.3 e documentacao da API nova"
git push
```

- [ ] **Step 5: Atualizar a issue #1**

Baixar o corpo (`gh issue view 1 --json body -q .body` para o scratchpad),
marcar `- [x]` nas quatro linhas da seção "v0.3" e nas linhas de colisão e
câmera da seção "Só wrapper", deixando a de timers desmarcada com a nota
`(cortado: num laço imediato `t = t + dt` já resolve)`. Depois
`gh issue edit 1 --body-file <arquivo>` e comentar:
`gh issue comment 1 --body "v0.3 implementado em main (fonte customizada, fullscreen/set_fps, roda + texto digitado + gamepad, polígonos, câmera e colisão). Sai no release v0.3.0."`

- [ ] **Step 6: Perguntar ao usuário sobre a tag**

Reportar o estado e perguntar se cria a tag `v0.3.0`. NÃO criar a tag sem
resposta explícita.
