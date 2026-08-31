# noxy_game_engine v0.2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar o pacote v0.2 da issue #1: alfa nas cores, comando de
imagem unificado (sub-retângulo, opacity, flip_x), áudio (WAV/OGG, efeitos +
música + volume global) e `text_width`.

**Architecture:** O protocolo de frame ganha o 4º componente de cor e um
comando `image` de aridade 12; áudio e `text_width` são exports avulsos fora
do `flip` (som não é por-frame). Efeitos são decodificados inteiros para PCM
na memória; música é streamed do arquivo com `audio.NewInfiniteLoop`. Só a
goroutine do SDK toca no estado de áudio (`concurrency = "single"`).

**Tech Stack:** Go 1.25, Ebiten v2.9.10 (`audio`, `audio/wav`,
`audio/vorbis`, `text/v2`, `vector`), SDK noxyplugin v0.1.0, wrapper Noxy.

**Spec:** `docs/superpowers/specs/2026-08-31-noxy-game-engine-v0.2-design.md`

## Global Constraints

- Commits em português no formato `tipo(escopo): descrição` (padrão do repo).
- Verificação Go de todo task: `go build ./... && go vet ./... && go test -race ./...` (rodar da raiz do repo).
- Quebra de API permitida (v0.x): `Color` ganha `a`, `draw_image_ex` ganha `opacity`/`flip_x` — exemplos e README saem atualizados no mesmo release.
- Áudio: sample rate **48000**, formatos WAV e OGG Vorbis, detecção por conteúdo (prefixo `OggS` → vorbis, senão wav). Nada de MP3.
- Áudio e `game_text_width` NÃO exigem `game_init`.
- Testes Go são headless: nunca criar `audio.Context` nem janela num teste.
- Binário local (Windows): `go build -o bin/noxy-plugin-game-windows-amd64.exe .`; exemplos rodam via junction: `cd /c/Users/sandr/Documents/noxy && ./noxy.exe ../noxy_game_engine/examples/<x>.nx`.
- A tag `v0.2.0` só é criada com confirmação explícita do usuário (fora deste plano).

---

### Task 1: Cor com alfa no protocolo

**Files:**
- Modify: `commands.go` (color3 → color4, aridades)
- Modify: `render.go` (color.NRGBA com alfa)
- Test: `commands_test.go`

**Interfaces:**
- Consumes: nada novo.
- Produces: `type color4 struct{ R, G, B, A uint8 }` (substitui `color3` no
  campo `command.color`); aridades novas `clear=5 rect=10 circle=9 line=10
  text=9` (o `image` muda na Task 2). Todo comando manda `r, g, b, a` onde
  antes mandava `r, g, b`.

- [ ] **Step 1: Atualizar os testes existentes e escrever os que faltam**

Em `commands_test.go`, atualização mecânica: em todo caso de teste, cada
cor de comando ganha um 4º componente (usar `255` nos casos válidos que hoje
têm 3 ints). Exemplos do padrão: `["clear", 10, 20, 30]` vira
`["clear", 10, 20, 30, 255]`; um `rect` válido vira
`["rect", 1.0, 2.0, 3.0, 4.0, 9, 8, 7, 255, 0]`. Mensagens de erro
esperadas que citam aridade mudam junto (ex.: `"clear" expects 5 elements`).
Casos de `image` ficam para a Task 2 — nesta task apenas compilam (ver Step
3). Acrescentar casos novos:

```go
func TestDecodeAlphaOutOfRange(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"clear", 0, 0, 0, 256}})
	if err == nil || !strings.Contains(err.Error(), "color component out of range 0..255, got 256") {
		t.Fatalf("want alpha range error, got %v", err)
	}
}

func TestDecodeAlphaAccepted(t *testing.T) {
	cmds, err := decodeFrame([]any{[]any{"rect", 1, 2, 3, 4, 10, 20, 30, 128, 0}})
	if err != nil {
		t.Fatal(err)
	}
	if cmds[0].color != (color4{10, 20, 30, 128}) {
		t.Fatalf("got %+v", cmds[0].color)
	}
}

func TestDecodeOldColorArityRejected(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"clear", 0, 0, 0}})
	if err == nil || !strings.Contains(err.Error(), `"clear" expects 5 elements, got 4`) {
		t.Fatalf("want arity error, got %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestDecode' -count=1`
Expected: FAIL (color4 não existe / aridades antigas).

- [ ] **Step 3: Implementar em commands.go e render.go**

`commands.go`:
- `type color3 struct{ R, G, B uint8 }` → `type color4 struct{ R, G, B, A uint8 }`;
  campo `command.color` vira `color4`; atualizar o comentário do struct
  (`clear: color` etc. seguem valendo).
- `arity`: `{"clear": 5, "rect": 10, "circle": 9, "line": 10, "text": 9, "image": 6}`
  (o 6 do `image` cai na Task 2).
- `reader.color`: loop `k < 4`, `var out [4]uint8`, retorna
  `color4{out[0], out[1], out[2], out[3]}` (a mensagem de erro
  `color component ...` não muda).
- Índices dos elementos após a cor sobem 1: `rect` thickness em 9, `circle`
  thickness em 8, `line` thickness em 9 (`text` não tem nada após a cor).

`render.go`, em `g.draw`:

```go
clr := color.NRGBA{c.color.R, c.color.G, c.color.B, c.color.A}
```

(única mudança nesta task — NRGBA respeita o alfa no blend source-over das
primitivas e do `ScaleWithColor` do texto; `screen.Fill` substitui os
pixels, o que é a semântica certa para `clear`).

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add commands.go render.go commands_test.go
git commit -m "feat(protocolo): cor com alfa (r,g,b,a) em todos os comandos"
```

---

### Task 2: Comando image unificado (sub-retângulo, opacity, flip_x)

**Files:**
- Modify: `commands.go` (image aridade 12, campos e leitores novos)
- Modify: `engine.go:172-179` (validação de sub-retângulo no handleFlip)
- Modify: `render.go` (SubImage, flip, opacity)
- Test: `commands_test.go`, `engine_test.go`

**Interfaces:**
- Consumes: `color4` e aridades da Task 1.
- Produces: comando `image` na forma
  `["image", id, sx, sy, sw, sh, x, y, scale, angle_deg, opacity, flip_x]`
  (aridade 12); campos novos em `command`:
  `srcX, srcY, srcW, srcH, opacity float64` e `flipX bool`. O wrapper (Task
  6) manda `0, 0, width, height` quando não há recorte e `1.0, false` como
  defaults de opacity/flip.

- [ ] **Step 1: Escrever os testes**

Em `commands_test.go`, atualizar os casos existentes de `image` para a forma
nova (ex.: `["image", int64(1), 5.0, 6.0, 2.0, 90.0]` vira
`["image", int64(1), 0, 0, 32, 32, 5.0, 6.0, 2.0, 90.0, 1.0, false]`, e
asserts dos campos `srcX/srcY/srcW/srcH/opacity/flipX`). Casos novos:

```go
func TestDecodeImageSub(t *testing.T) {
	cmds, err := decodeFrame([]any{[]any{"image", int64(3), 32.0, 0.0, 32.0, 48.0, 10.0, 20.0, 2.0, 45.0, 0.5, true}})
	if err != nil {
		t.Fatal(err)
	}
	c := cmds[0]
	if c.image != 3 || c.srcX != 32 || c.srcY != 0 || c.srcW != 32 || c.srcH != 48 ||
		c.x != 10 || c.y != 20 || c.scale != 2 || c.angle != 45 || c.opacity != 0.5 || !c.flipX {
		t.Fatalf("got %+v", c)
	}
}

func TestDecodeImageBadOpacity(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"image", int64(1), 0, 0, 8, 8, 0.0, 0.0, 1.0, 0.0, 1.5, false}})
	if err == nil || !strings.Contains(err.Error(), "opacity out of range 0..1, got 1.5") {
		t.Fatalf("want opacity error, got %v", err)
	}
}

func TestDecodeImageBadFlip(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"image", int64(1), 0, 0, 8, 8, 0.0, 0.0, 1.0, 0.0, 1.0, int64(1)}})
	if err == nil || !strings.Contains(err.Error(), "expected bool, got int") {
		t.Fatalf("want bool error, got %v", err)
	}
}

func TestDecodeImageBadSourceSize(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"image", int64(1), 0, 0, 0, 8, 0.0, 0.0, 1.0, 0.0, 1.0, false}})
	if err == nil || !strings.Contains(err.Error(), "source size must be positive, got 0") {
		t.Fatalf("want size error, got %v", err)
	}
}

func TestDecodeImageOldArityRejected(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"image", int64(1), 5.0, 6.0, 2.0, 90.0}})
	if err == nil || !strings.Contains(err.Error(), `"image" expects 12 elements, got 6`) {
		t.Fatalf("want arity error, got %v", err)
	}
}
```

Em `engine_test.go`, junto do teste existente de id de imagem desconhecido
(procurar por `unknown image`), acrescentar — usando o helper que os testes
existentes já usam para registrar uma imagem no engine (se registram via
`e.images[1] = &imageEntry{src: ...}` com uma `image.NewRGBA`, seguir o
mesmo padrão; a imagem de teste deve ter 64x32):

```go
func TestFlipRejectsSourceRectOutsideImage(t *testing.T) {
	e := newEngine()
	e.started = true
	e.images[2] = &imageEntry{src: image.NewRGBA(image.Rect(0, 0, 64, 32))}
	frame := []any{[]any{"image", int64(2), 40.0, 0.0, 32.0, 32.0, 0.0, 0.0, 1.0, 0.0, 1.0, false}}
	done := make(chan error, 1)
	go func() { _, err := e.handleFlip(context.Background(), frame); done <- err }()
	err := <-done
	if err == nil || !strings.Contains(err.Error(), `command 0: element 2 of "image": source rect 40,0 32x32 outside image 2 (64x32)`) {
		t.Fatalf("want source rect error, got %v", err)
	}
}
```

(o handleFlip com frame inválido retorna antes de publicar o frame, então
não precisa de tick; o goroutine é só por consistência com o padrão do
arquivo — se os testes existentes chamam `handleFlip` direto, chamar direto.)

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestDecodeImage|TestFlipRejects' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar**

`commands.go`:
- `arity["image"] = 12`.
- `command` ganha `srcX, srcY, srcW, srcH, opacity float64` e `flipX bool`
  (na linha dos campos float e um campo bool novo); comentário do struct:
  `image: image sx sy sw sh x y scale angle opacity flipX`.
- decode do `image`:

```go
case "image":
	c.kind = cmdImage
	c.image = r.integer(1)
	c.srcX, c.srcY = r.num(2), r.num(3)
	c.srcW, c.srcH = r.positive(4), r.positive(5)
	c.x, c.y = r.num(6), r.num(7)
	c.scale, c.angle = r.num(8), r.num(9)
	c.opacity = r.opacity(10)
	c.flipX = r.boolean(11)
```

- leitores novos no `reader`:

```go
func (r *reader) positive(i int) float64 {
	v := r.num(i)
	if r.err == nil && v <= 0 {
		r.fail(i, "source size must be positive, got %g", v)
	}
	return v
}

func (r *reader) opacity(i int) float64 {
	v := r.num(i)
	if r.err == nil && (v < 0 || v > 1) {
		r.fail(i, "opacity out of range 0..1, got %g", v)
	}
	return v
}

func (r *reader) boolean(i int) bool {
	b, ok := r.parts[i].(bool)
	if !ok {
		r.fail(i, "expected bool, got %s", typeName(r.parts[i]))
	}
	return b
}
```

`engine.go`, no loop de validação do `handleFlip` (substitui o corpo do
`if c.kind == cmdImage`):

```go
for i, c := range cmds {
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
```

`render.go`, caso `cmdImage` (substitui o existente; import novo `"image"`):

```go
case cmdImage:
	entry, err := g.e.image(c.image) // ids e recortes validados no flip
	if err != nil {
		return
	}
	if entry.tex == nil {
		entry.tex = ebiten.NewImageFromImage(entry.src)
	}
	sx, sy := int(c.srcX), int(c.srcY)
	sw, sh := int(c.srcW), int(c.srcH)
	sub := entry.tex.SubImage(image.Rect(sx, sy, sx+sw, sy+sh)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	if c.flipX {
		op.GeoM.Scale(-1, 1)
		op.GeoM.Translate(float64(sw), 0)
	}
	op.GeoM.Scale(c.scale, c.scale)
	if c.angle != 0 {
		w := float64(sw) * c.scale
		h := float64(sh) * c.scale
		op.GeoM.Translate(-w/2, -h/2)
		op.GeoM.Rotate(c.angle * math.Pi / 180)
		op.GeoM.Translate(w/2, h/2)
	}
	op.GeoM.Translate(c.x, c.y)
	if c.opacity < 1 {
		op.ColorScale.ScaleAlpha(float32(c.opacity))
	}
	screen.DrawImage(sub, op)
```

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add commands.go engine.go render.go commands_test.go engine_test.go
git commit -m "feat(protocolo): image unificado — sub-retangulo, opacity e flip_x"
```

---

### Task 3: text_width

**Files:**
- Modify: `font.go` (função `textWidth`)
- Modify: `main.go` (handler `game_text_width`)
- Modify: `noxy_ext.toml` (export)
- Test: `font_test.go` (novo)

**Interfaces:**
- Consumes: `fontFace(size float64) text.Face` de `font.go` (já existe, cache
  com mutex).
- Produces: `textWidth(s string, size float64) float64` e
  `handleTextWidth(ctx context.Context, s string, size int64) (float64, error)`;
  export `game_text_width(string, int) -> float`. O wrapper (Task 6) chama
  `game_text_width(s, size)`.

- [ ] **Step 1: Escrever os testes**

`font_test.go` (novo):

```go
// font_test.go — textWidth é headless: mede com a face, sem janela.
package main

import (
	"sync"
	"testing"
)

func TestTextWidth(t *testing.T) {
	if w := textWidth("", 16); w != 0 {
		t.Fatalf("empty string: want 0, got %g", w)
	}
	ab := textWidth("ab", 16)
	abc := textWidth("abc", 16)
	if !(ab > 0 && abc > ab) {
		t.Fatalf("want 0 < %g < %g", ab, abc)
	}
	if big := textWidth("ab", 32); big <= ab {
		t.Fatalf("size 32 (%g) should be wider than 16 (%g)", big, ab)
	}
}

func TestFontFaceConcurrent(t *testing.T) { // relevante com -race
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				textWidth("x", float64(10+j%5))
			}
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestTextWidth|TestFontFace' -count=1`
Expected: FAIL (`textWidth` não existe).

- [ ] **Step 3: Implementar**

`font.go` (no fim do arquivo):

```go
// textWidth mede a largura de s na fonte embutida, em px. Headless: não
// precisa de janela nem de game_init.
func textWidth(s string, size float64) float64 {
	return text.Advance(s, fontFace(size))
}
```

`main.go`: registrar junto dos outros handlers (o handler é função livre —
não toca no engine):

```go
p.Handle("game_text_width", noxyplugin.Func2(handleTextWidth))
```

e no fim do arquivo:

```go
// handleTextWidth: game_text_width(s, size) -> float. Não exige game_init.
func handleTextWidth(ctx context.Context, s string, size int64) (float64, error) {
	if size <= 0 {
		return 0, fmt.Errorf("size must be positive, got %d", size)
	}
	return textWidth(s, float64(size)), nil
}
```

(import novo em `main.go`: `"context"`.)

`noxy_ext.toml` (no fim):

```toml
[[export]]
name = "game_text_width"        # (s, size) -> largura em px na fonte embutida
params = ["string", "int"]
returns = "float"
```

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add font.go font_test.go main.go noxy_ext.toml
git commit -m "feat(texto): game_text_width — medida de texto na fonte embutida"
```

---

### Task 4: Áudio — decodificação e registro (headless)

**Files:**
- Create: `audio.go`
- Test: `audio_test.go` (novo)

**Interfaces:**
- Consumes: nada do engine (áudio é independente da janela).
- Produces: `newAudioEngine() *audioEngine`;
  `decodeAudio(r io.ReadSeeker) (io.ReadSeeker, error)` (stream PCM 16-bit
  estéreo 48 kHz); `(*audioEngine).loadSound(path string) (int64, error)`;
  `(*audioEngine).setVolume(v float64) error`; constante `sampleRate = 48000`.
  A Task 5 adiciona playSound/playMusic/stopMusic e os handlers.

- [ ] **Step 1: Escrever os testes**

`audio_test.go` (novo). O WAV de teste é gerado em código (16-bit mono
8000 Hz; o decoder resampleia para 48 kHz estéreo) — sem testdata commitado.
OGG válido não dá para gerar em Go puro (não há encoder), então o caminho
vorbis é testado pela detecção do prefixo `OggS` com payload inválido.

```go
// audio_test.go — decodificação e registro, headless: nenhum teste cria
// audio.Context (não há dispositivo de som no CI).
package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wavBytes gera um WAV PCM 16-bit mono 8000 Hz com nSamples amostras.
func wavBytes(nSamples int) []byte {
	var b bytes.Buffer
	dataLen := nSamples * 2
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+dataLen))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&b, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&b, binary.LittleEndian, uint32(8000))
	binary.Write(&b, binary.LittleEndian, uint32(8000*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(dataLen))
	for i := 0; i < nSamples; i++ {
		binary.Write(&b, binary.LittleEndian, int16(math.Sin(float64(i)*0.3)*8000))
	}
	return b.Bytes()
}

func writeTempWav(t *testing.T, nSamples int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.wav")
	if err := os.WriteFile(path, wavBytes(nSamples), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDecodeAudioWav(t *testing.T) {
	stream, err := decodeAudio(bytes.NewReader(wavBytes(800)))
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	// 800 amostras a 8 kHz = 0.1 s → 48000*0.1 amostras estéreo 16-bit
	if len(pcm) == 0 || len(pcm)%4 != 0 {
		t.Fatalf("PCM inesperado: %d bytes", len(pcm))
	}
}

func TestDecodeAudioOggInvalid(t *testing.T) {
	_, err := decodeAudio(bytes.NewReader([]byte("OggS garbage garbage")))
	if err == nil || !strings.Contains(err.Error(), "ogg") {
		t.Fatalf("want ogg error, got %v", err)
	}
}

func TestDecodeAudioGarbage(t *testing.T) {
	_, err := decodeAudio(bytes.NewReader([]byte("not audio at all")))
	if err == nil || !strings.Contains(err.Error(), "wav") {
		t.Fatalf("want wav error, got %v", err)
	}
}

func TestDecodeAudioTooShort(t *testing.T) {
	_, err := decodeAudio(bytes.NewReader([]byte("ab")))
	if err == nil {
		t.Fatal("want error for short input")
	}
}

func TestLoadSoundRegisters(t *testing.T) {
	a := newAudioEngine()
	id1, err := a.loadSound(writeTempWav(t, 400))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := a.loadSound(writeTempWav(t, 400))
	if err != nil {
		t.Fatal(err)
	}
	if id1 != 1 || id2 != 2 {
		t.Fatalf("want ids 1,2, got %d,%d", id1, id2)
	}
	if len(a.sounds[id1]) == 0 {
		t.Fatal("PCM não registrado")
	}
	if a.ctx != nil {
		t.Fatal("loadSound não deve criar o audio.Context")
	}
}

func TestLoadSoundMissingFile(t *testing.T) {
	a := newAudioEngine()
	if _, err := a.loadSound(filepath.Join(t.TempDir(), "nope.wav")); err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestSetVolume(t *testing.T) {
	a := newAudioEngine()
	if err := a.setVolume(0.5); err != nil || a.volume != 0.5 {
		t.Fatalf("got err=%v volume=%g", err, a.volume)
	}
	for _, v := range []float64{-0.1, 1.5} {
		if err := a.setVolume(v); err == nil || !strings.Contains(err.Error(), "volume must be between 0 and 1") {
			t.Fatalf("volume %g: want range error, got %v", v, err)
		}
	}
	if a.ctx != nil {
		t.Fatal("setVolume não deve criar o audio.Context")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestDecodeAudio|TestLoadSound|TestSetVolume' -count=1`
Expected: FAIL (audio.go não existe).

- [ ] **Step 3: Implementar audio.go**

```go
// audio.go — efeitos e música, exports avulsos fora do frame (som não é
// por-frame; ~45 µs por chamada é imperceptível). Efeitos viram PCM na
// memória; música é streamed do arquivo em loop. Só a goroutine do SDK
// toca aqui (concurrency = "single"); o mutex é defensivo. O audio.Context
// nasce preguiçosamente na primeira reprodução — nada aqui exige game_init.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const sampleRate = 48000

type audioEngine struct {
	mu     sync.Mutex
	ctx    *audio.Context // criado na primeira reprodução (singleton do processo)
	sounds map[int64][]byte // PCM 16-bit estéreo 48 kHz, decodificado inteiro
	nextID int64
	music  *musicEntry
	volume float64 // global 0..1
}

type musicEntry struct {
	player *audio.Player
	file   *os.File
}

func newAudioEngine() *audioEngine {
	return &audioEngine{sounds: map[int64][]byte{}, volume: 1}
}

// decodeAudio detecta o formato pelo conteúdo: "OggS" → vorbis, senão WAV.
func decodeAudio(r io.ReadSeeker) (io.ReadSeeker, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, errors.New("not a WAV or OGG file: too short")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if bytes.Equal(magic[:], []byte("OggS")) {
		s, err := vorbis.DecodeWithSampleRate(sampleRate, r)
		if err != nil {
			return nil, fmt.Errorf("ogg: %w", err)
		}
		return s, nil
	}
	s, err := wav.DecodeWithSampleRate(sampleRate, r)
	if err != nil {
		return nil, fmt.Errorf("wav: %w", err)
	}
	return s, nil
}

// loadSound decodifica o arquivo inteiro para a memória e devolve o id.
func (a *audioEngine) loadSound(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	stream, err := decodeAudio(f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	pcm, err := io.ReadAll(stream)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextID++
	a.sounds[a.nextID] = pcm
	return a.nextID, nil
}

// setVolume aplica na música imediatamente; sons já disparados terminam no
// volume em que começaram.
func (a *audioEngine) setVolume(v float64) error {
	if v < 0 || v > 1 {
		return fmt.Errorf("volume must be between 0 and 1, got %g", v)
	}
	a.mu.Lock()
	a.volume = v
	m := a.music
	a.mu.Unlock()
	if m != nil {
		m.player.SetVolume(v)
	}
	return nil
}
```

- [ ] **Step 4: go mod tidy e verificar**

Run: `go mod tidy && go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS (o tidy puxa `github.com/jfreymuth/oggvorbis` e afins como
dependências do `audio/vorbis`).

- [ ] **Step 5: Commit**

```bash
git add audio.go audio_test.go go.mod go.sum
git commit -m "feat(audio): decodificacao WAV/OGG e registro de sons (headless)"
```

---

### Task 5: Áudio — reprodução, música e exports

**Files:**
- Modify: `audio.go` (ensureCtx, playSound, playMusic, stopMusic, handlers)
- Modify: `main.go` (registro dos handlers)
- Modify: `noxy_ext.toml` (5 exports)
- Test: `audio_test.go`

**Interfaces:**
- Consumes: `audioEngine`, `decodeAudio`, `loadSound`, `setVolume` da Task 4.
- Produces: exports `game_load_sound(string) -> map[string]any` (`{"id"}`),
  `game_play_sound(int) -> void`, `game_play_music(string) -> void`,
  `game_stop_music() -> void`, `game_set_volume(float) -> void`. O wrapper
  (Task 6) chama exatamente esses nomes.

- [ ] **Step 1: Escrever os testes**

Em `audio_test.go` (só caminhos que não criam contexto):

```go
func TestPlaySoundUnknownID(t *testing.T) {
	a := newAudioEngine()
	err := a.playSound(7)
	if err == nil || !strings.Contains(err.Error(), "unknown sound 7 (not returned by load_sound)") {
		t.Fatalf("want unknown sound error, got %v", err)
	}
	if a.ctx != nil {
		t.Fatal("id inválido não deve criar o audio.Context")
	}
}

func TestPlayMusicMissingFile(t *testing.T) {
	a := newAudioEngine()
	if err := a.playMusic(filepath.Join(t.TempDir(), "nope.ogg")); err == nil {
		t.Fatal("want error for missing file")
	}
	if a.ctx != nil {
		t.Fatal("arquivo inválido não deve criar o audio.Context")
	}
}

func TestStopMusicIdempotent(t *testing.T) {
	a := newAudioEngine()
	a.stopMusic()
	a.stopMusic() // sem música: não pode explodir
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./... -run 'TestPlaySound|TestPlayMusic|TestStopMusic' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar**

`audio.go` (continuação):

```go
// ensureCtx cria o audio.Context na primeira reprodução. Chamar só com a
// entrada já validada: o contexto é um singleton do processo.
func (a *audioEngine) ensureCtx() *audio.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil {
		a.ctx = audio.NewContext(sampleRate)
	}
	return a.ctx
}

// playSound toca o som do início, com sobreposição livre (um player novo
// por play, fire-and-forget; o mixer segura o player até o fim).
func (a *audioEngine) playSound(id int64) error {
	a.mu.Lock()
	pcm, ok := a.sounds[id]
	vol := a.volume
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown sound %d (not returned by load_sound)", id)
	}
	p := a.ensureCtx().NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
	return nil
}

// playMusic toca o arquivo em streaming, em loop infinito; com música já
// tocando, troca (para e fecha a anterior).
func (a *audioEngine) playMusic(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	stream, err := decodeAudio(f)
	if err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	length, err := stream.Seek(0, io.SeekEnd)
	if err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if _, err := stream.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	p, err := a.ensureCtx().NewPlayer(audio.NewInfiniteLoop(stream, length))
	if err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	a.mu.Lock()
	old := a.music
	a.music = &musicEntry{player: p, file: f}
	vol := a.volume
	a.mu.Unlock()
	if old != nil {
		old.player.Close()
		old.file.Close()
	}
	p.SetVolume(vol)
	p.Play()
	return nil
}

// stopMusic para e fecha a música. Idempotente.
func (a *audioEngine) stopMusic() {
	a.mu.Lock()
	m := a.music
	a.music = nil
	a.mu.Unlock()
	if m != nil {
		m.player.Close()
		m.file.Close()
	}
}

// ---- handlers (goroutine do SDK) ----

func (a *audioEngine) handleLoadSound(ctx context.Context, path string) (map[string]any, error) {
	id, err := a.loadSound(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (a *audioEngine) handlePlaySound(ctx context.Context, id int64) (any, error) {
	return nil, a.playSound(id)
}

func (a *audioEngine) handlePlayMusic(ctx context.Context, path string) (any, error) {
	return nil, a.playMusic(path)
}

func (a *audioEngine) handleStopMusic(ctx context.Context) (any, error) {
	a.stopMusic()
	return nil, nil
}

func (a *audioEngine) handleSetVolume(ctx context.Context, v float64) (any, error) {
	return nil, a.setVolume(v)
}
```

(import novo em `audio.go`: `"context"`.)

`main.go`, depois dos handlers do engine:

```go
a := newAudioEngine()
p.Handle("game_load_sound", noxyplugin.Func1(a.handleLoadSound))
p.Handle("game_play_sound", noxyplugin.Func1(a.handlePlaySound))
p.Handle("game_play_music", noxyplugin.Func1(a.handlePlayMusic))
p.Handle("game_stop_music", noxyplugin.Func0(a.handleStopMusic))
p.Handle("game_set_volume", noxyplugin.Func1(a.handleSetVolume))
```

`noxy_ext.toml` (antes do export de `game_text_width`, mantendo os exports
de áudio juntos):

```toml
[[export]]
name = "game_load_sound"        # (path) -> {"id"}; WAV ou OGG inteiro na memoria
params = ["string"]
returns = "map[string]any"
stateful = true

[[export]]
name = "game_play_sound"        # (id): toca do inicio, sobreposicao livre
params = ["int"]
returns = "void"
stateful = true

[[export]]
name = "game_play_music"        # (path): streaming em loop; troca a musica atual
params = ["string"]
returns = "void"
stateful = true

[[export]]
name = "game_stop_music"
params = []
returns = "void"
stateful = true

[[export]]
name = "game_set_volume"        # (v 0..1): musica na hora, sons no proximo play
params = ["float"]
returns = "void"
stateful = true
```

Atualizar também o comentário `capabilities` do topo do toml para citar áudio
(ex.: `capabilities = ["display", "fs"] # janela; load_image/load_sound leem arquivos`).

- [ ] **Step 4: Verificar**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add audio.go audio_test.go main.go noxy_ext.toml
git commit -m "feat(audio): play/música em loop/volume global e exports"
```

---

### Task 6: Wrapper v0.2 e smoke local

**Files:**
- Modify: `noxy_game_engine.nx`
- Modify: `examples/smoke.nx` (só se usar Color(...) posicional — verificar)

**Interfaces:**
- Consumes: os exports das Tasks 3 e 5 e o protocolo das Tasks 1–2.
- Produces: API pública v0.2 — `Color{r,g,b,a}`, `rgba`, `Sound{id}`,
  `load_sound`, `play`, `play_music`, `stop_music`, `set_volume`,
  `text_width`, `draw_image_sub`, `draw_image_sub_ex`, e a assinatura nova
  `draw_image_ex(img, x, y, scale, angle_deg, opacity, flip_x)`. Os exemplos
  (Task 7) usam exatamente esses nomes.

- [ ] **Step 1: Editar noxy_game_engine.nx**

Mudanças (o restante do arquivo fica como está):

`Color` e constantes:

```noxy
struct Color
    r: int
    g: int
    b: int
    a: int
end

struct Sound
    id: int
end

let BLACK: Color = Color(0, 0, 0, 255)
let WHITE: Color = Color(255, 255, 255, 255)
let RED: Color = Color(255, 0, 0, 255)
let GREEN: Color = Color(0, 255, 0, 255)
let BLUE: Color = Color(0, 0, 255, 255)
let YELLOW: Color = Color(255, 255, 0, 255)

func rgb(r: int, g: int, b: int) -> Color
    return Color(r, g, b, 255)
end

// rgba: alfa 0 (transparente) a 255 (opaco).
func rgba(r: int, g: int, b: int, a: int) -> Color
    return Color(r, g, b, a)
end
```

Desenho — todo append de cor ganha `c.a` (mesma posição relativa: logo após
`c.b`):

```noxy
func clear(c: Color) -> void
    append(ref _cmds, ["clear", c.r, c.g, c.b, c.a])
end

func draw_rect(x: float, y: float, w: float, h: float, c: Color) -> void
    append(ref _cmds, ["rect", x, y, w, h, c.r, c.g, c.b, c.a, 0])
end

func draw_rect_outline(x: float, y: float, w: float, h: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["rect", x, y, w, h, c.r, c.g, c.b, c.a, thickness])
end

func draw_circle(cx: float, cy: float, radius: float, c: Color) -> void
    append(ref _cmds, ["circle", cx, cy, radius, c.r, c.g, c.b, c.a, 0])
end

func draw_circle_outline(cx: float, cy: float, radius: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["circle", cx, cy, radius, c.r, c.g, c.b, c.a, thickness])
end

func draw_line(x1: float, y1: float, x2: float, y2: float, c: Color, thickness: float) -> void
    append(ref _cmds, ["line", x1, y1, x2, y2, c.r, c.g, c.b, c.a, thickness])
end

func draw_text(s: string, x: float, y: float, size: int, c: Color) -> void
    append(ref _cmds, ["text", s, x, y, size, c.r, c.g, c.b, c.a])
end
```

Imagens (substituem `draw_image`/`draw_image_ex` atuais):

```noxy
// draw_image draws img with its top-left corner at (x, y), unscaled.
func draw_image(img: Image, x: float, y: float) -> void
    append(ref _cmds, ["image", img.id, 0, 0, img.width, img.height, x, y, 1.0, 0.0, 1.0, false])
end

// draw_image_ex: scale, rotação em graus em torno do centro da imagem
// escalada, opacity 0..1 e espelhamento horizontal.
func draw_image_ex(img: Image, x: float, y: float, scale: float, angle_deg: float, opacity: float, flip_x: bool) -> void
    append(ref _cmds, ["image", img.id, 0, 0, img.width, img.height, x, y, scale, angle_deg, opacity, flip_x])
end

// draw_image_sub desenha o sub-retângulo (sx, sy, sw, sh) de img — o frame
// de um sprite sheet — com o canto superior esquerdo em (x, y).
func draw_image_sub(img: Image, sx: float, sy: float, sw: float, sh: float, x: float, y: float) -> void
    append(ref _cmds, ["image", img.id, sx, sy, sw, sh, x, y, 1.0, 0.0, 1.0, false])
end

// draw_image_sub_ex: sub-retângulo com scale, rotação, opacity e flip_x.
func draw_image_sub_ex(img: Image, sx: float, sy: float, sw: float, sh: float, x: float, y: float, scale: float, angle_deg: float, opacity: float, flip_x: bool) -> void
    append(ref _cmds, ["image", img.id, sx, sy, sw, sh, x, y, scale, angle_deg, opacity, flip_x])
end
```

Áudio e texto (seção nova antes de `---- input ----`):

```noxy
// ---- audio (chamadas diretas; não passam pelo frame) ----

// load_sound decodifica um WAV ou OGG inteiro para a memória.
func load_sound(path: string) -> Sound
    let info: map[string, any] = game_load_sound(path)
    let id: int = info["id"]
    return Sound(id)
end

// play toca o som do início, com sobreposição livre.
func play(s: Sound) -> void
    game_play_sound(s.id)
end

// play_music toca o arquivo (WAV/OGG) em streaming, em loop, até
// stop_music. Chamada com música tocando troca a música.
func play_music(path: string) -> void
    game_play_music(path)
end

func stop_music() -> void
    game_stop_music()
end

// set_volume: volume global 0..1. Aplica na música na hora e nos próximos
// play; sons já disparados terminam no volume em que começaram.
func set_volume(v: float) -> void
    game_set_volume(v)
end

// text_width mede a largura de s em px na fonte embutida (mesma medida do
// draw_text). Num laço quente, medir uma vez e guardar.
func text_width(s: string, size: int) -> float
    let w: float = game_text_width(s, size)
    return w
end
```

Atualizar o comentário do topo do arquivo: a lista de natives ganha os 6
novos (game_load_sound, game_play_sound, game_play_music, game_stop_music,
game_set_volume, game_text_width).

- [ ] **Step 2: Rebuild do binário e smoke**

Verificar antes se `examples/smoke.nx` constrói `Color(...)` posicional
(se sim, adicionar o 4º campo; se usa `rgb()`/constantes, nada muda).

Run (Bash):
```bash
cd /c/Users/sandr/Documents/noxy_game_engine && go build -o bin/noxy-plugin-game-windows-amd64.exe . && cd /c/Users/sandr/Documents/noxy && ./noxy.exe ../noxy_game_engine/examples/smoke.nx
```
Expected: janela abre ~0,5 s e imprime `ok` (a junction em noxy_libs já
aponta para o checkout).

- [ ] **Step 3: Testar as APIs novas com um script descartável**

Criar no scratchpad (não no repo) um `v02_check.nx` que exercita tudo de
novo de uma vez, rodando ~60 frames sem interação:

```noxy
use github_com.estevaofon.noxy_game_engine as game

game.init(320, 240, "v0.2 check")
let w: float = game.text_width("Game over", 30)
print(f"text_width: {w}")
let img: game.Image = game.load_image("../noxy_game_engine/examples/sprite.png")
let snd: game.Sound = game.load_sound("<scratchpad>/beep.wav")
game.set_volume(0.4)
game.play(snd)
let i = 0
while game.running() && i < 60 do
    game.clear(game.rgba(20, 20, 60, 255))
    game.draw_rect(10.0, 10.0, 100.0, 60.0, game.rgba(255, 0, 0, 120))
    game.draw_image_sub(img, 0.0, 0.0, 16.0, 16.0, 150.0, 30.0)
    game.draw_image_ex(img, 150.0, 100.0, 2.0, 30.0, 0.5, true)
    game.draw_image_sub_ex(img, 8.0, 8.0, 16.0, 16.0, 40.0, 120.0, 3.0, 0.0, 1.0, true)
    game.draw_text("alpha ok", 10.0, 200.0, 16, game.rgba(255, 255, 255, 160))
    game.flip()
    i = i + 1
end
game.quit()
print("v0.2 ok")
```

O `beep.wav` do scratchpad: gerar com o mesmo formato do `wavBytes` do
teste (um `go run` de script descartável no scratchpad, 0,2 s de senoide).
Substituir `<scratchpad>` pelo caminho real. Atenção aos caminhos: o cwd é
`C:\Users\sandr\Documents\noxy`.

Run: `cd /c/Users/sandr/Documents/noxy && ./noxy.exe <scratchpad>/v02_check.nx`
Expected: imprime `text_width: <número > 0>`, toca um beep, mostra
retângulo translúcido, recortes e flip, termina com `v0.2 ok`, exit 0.

- [ ] **Step 4: Commit**

```bash
git add noxy_game_engine.nx
git commit -m "feat(wrapper): API v0.2 — rgba, sprite sheet, audio e text_width"
```

(incluir `examples/smoke.nx` no add se tiver mudado.)

---

### Task 7: Exemplos — sprite, animation e flappy com som

**Files:**
- Modify: `examples/sprite.nx` (assinatura nova + demo de opacity/flip)
- Create: `examples/animation.nx`, `examples/walker.png`
- Modify: `examples/flappy_bird.nx` (sons, painel translúcido, texto centrado)
- Create: `examples/flap.wav`, `examples/score.wav`, `examples/hit.wav`

**Interfaces:**
- Consumes: a API v0.2 do wrapper (Task 6).
- Produces: nada para outras tasks; assets commitados no repo.

- [ ] **Step 1: Atualizar sprite.nx**

As três chamadas `draw_image_ex` existentes ganham `, 1.0, false`; a que
segue o mouse vira uma demo de opacity+flip. Trecho novo do laço:

```noxy
    game.draw_image(sprite, 20.0, 20.0)
    game.draw_image_ex(sprite, 100.0, 20.0, 3.0, 0.0, 1.0, false)
    game.draw_image_ex(sprite, 250.0, 100.0, 2.0, angle, 1.0, false)
    let m: game.Point = game.mouse_pos()
    game.draw_image_ex(sprite, m.x - 16.0, m.y - 16.0, 1.0, 0.0, 0.5, true)
```

(e o comentário do topo menciona a cópia translúcida/espelhada no mouse.)

- [ ] **Step 2: Gerar walker.png e os WAVs**

Script Go descartável no scratchpad (rodar com `go run` de dentro do módulo
do scratchpad ou com `GO111MODULE=off`; mais simples: criar
`<scratchpad>/gen/main.go` com `go mod init gen` no diretório):

```go
// gen/main.go — gera examples/walker.png (4 frames 32x32 lado a lado de um
// boneco andando, cores chapadas, fundo transparente) e os 3 WAVs do flappy.
package main

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	out := os.Args[1] // caminho da pasta examples/
	writeWalker(out + "/walker.png")
	writeWav(out+"/flap.wav", 0.12, func(t float64) float64 {
		return math.Sin(2*math.Pi*(500+900*t/0.12)*t) * (1 - t/0.12)
	})
	writeWav(out+"/score.wav", 0.18, func(t float64) float64 {
		f := 880.0
		if t > 0.09 {
			f = 1320.0
		}
		return math.Sin(2*math.Pi*f*t) * 0.8
	})
	writeWav(out+"/hit.wav", 0.25, func(t float64) float64 {
		return math.Sin(2*math.Pi*110*t) * math.Exp(-8*t)
	})
}

func writeWalker(path string) {
	const fw, fh, n = 32, 32, 4
	img := image.NewNRGBA(image.Rect(0, 0, fw*n, fh))
	body := color.NRGBA{80, 120, 220, 255}
	skin := color.NRGBA{240, 200, 160, 255}
	// pernas por frame: aberto, meio, fechado, meio (deslocamento em px)
	legSpread := []int{6, 3, 0, 3}
	for f := 0; f < n; f++ {
		ox := f * fw
		fillRect(img, ox+12, 4, 8, 8, skin)    // cabeça
		fillRect(img, ox+10, 12, 12, 10, body) // tronco
		s := legSpread[f]
		fillRect(img, ox+11-s/2, 22, 4, 8, body) // perna esquerda
		fillRect(img, ox+17+s/2, 22, 4, 8, body) // perna direita
	}
	fl, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer fl.Close()
	if err := png.Encode(fl, img); err != nil {
		panic(err)
	}
}

func fillRect(img *image.NRGBA, x, y, w, h int, c color.NRGBA) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			img.SetNRGBA(i, j, c)
		}
	}
}

func writeWav(path string, dur float64, sample func(t float64) float64) {
	const rate = 44100
	n := int(dur * rate)
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dataLen := n * 2
	f.WriteString("RIFF")
	binary.Write(f, binary.LittleEndian, uint32(36+dataLen))
	f.WriteString("WAVEfmt ")
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint32(rate))
	binary.Write(f, binary.LittleEndian, uint32(rate*2))
	binary.Write(f, binary.LittleEndian, uint16(2))
	binary.Write(f, binary.LittleEndian, uint16(16))
	f.WriteString("data")
	binary.Write(f, binary.LittleEndian, uint32(dataLen))
	for i := 0; i < n; i++ {
		t := float64(i) / rate
		binary.Write(f, binary.LittleEndian, int16(sample(t)*20000))
	}
}
```

Run: `cd <scratchpad>/gen && go mod init gen 2>/dev/null; go run . /c/Users/sandr/Documents/noxy_game_engine/examples`
Expected: `walker.png` (128x32) e os 3 `.wav` criados em examples/.

- [ ] **Step 3: Criar animation.nx**

```noxy
// animation.nx — sprite sheet: examples/walker.png tem 4 frames de 32x32
// lado a lado; draw_image_sub recorta o frame, flip_x vira o boneco.
// Rode da raiz do repositório: noxy examples/animation.nx
use github_com.estevaofon.noxy_game_engine as game

let FRAME = 32.0
let FRAMES = 4
let SPEED = 120.0            // px/s
let FPS_ANIM = 10.0          // frames de animação por segundo

game.init(480, 200, "Animation")
let sheet: game.Image = game.load_image("examples/walker.png")

let x = 40.0
let facing_right = true
let t = 0.0
while game.running() do
    if game.key_pressed("escape") then
        game.stop()
    end
    let dt: float = game.delta()
    t = t + dt
    if facing_right then
        x = x + SPEED * dt
        if x > 400.0 then facing_right = false end
    else
        x = x - SPEED * dt
        if x < 40.0 then facing_right = true end
    end
    let frame = to_int(t * FPS_ANIM) % FRAMES

    game.clear(game.rgb(30, 30, 30))
    game.draw_line(0.0, 160.0, 480.0, 160.0, game.WHITE, 2.0)
    // 4x de escala: draw_image_sub_ex com o frame recortado
    game.draw_image_sub_ex(sheet, to_float(frame) * FRAME, 0.0, FRAME, FRAME, x, 32.0, 4.0, 0.0, 1.0, !facing_right)
    game.draw_text("Escape to quit", 10.0, 176.0, 14, game.WHITE)
    game.flip()
end
game.quit()
```

(Atenção à regra de memória: `let` inicializado por chamada com alias
precisa de anotação de tipo — já aplicada acima.)

- [ ] **Step 4: Sons e polish no flappy_bird.nx**

Mudanças em `examples/flappy_bird.nx`:

Depois do `game.init`:

```noxy
let SND_FLAP: game.Sound = game.load_sound("examples/flap.wav")
let SND_SCORE: game.Sound = game.load_sound("examples/score.wav")
let SND_HIT: game.Sound = game.load_sound("examples/hit.wav")
```

Em `update`, no ponto que conta o score (`score = score + 1`), acrescentar
`game.play(SND_SCORE)`. A morte toca uma vez só — mudar o fim de `update`:

```noxy
    if bird_y + BIRD_R > H - GROUND || bird_y - BIRD_R < 0.0 then
        alive = false
    end
    if !alive then
        game.play(SND_HIT)
        if score > best then best = score end
    end
```

fica correto porque `update` só roda com `alive == true` no laço principal
(a checagem `hits` põe `alive = false` dentro do mesmo update). Conferir no
laço principal que `update` não roda após a morte (hoje: `if !alive then
... else ... update ... end` — ok).

No flap (laço principal), acrescentar `game.play(SND_FLAP)` junto de
`bird_vy = FLAP`.

Painel de game over translúcido e textos centrados (substitui o bloco
`if !alive then` do `draw`):

```noxy
    if !alive then
        game.draw_rect(60.0, 230.0, 280.0, 120.0, game.rgba(0, 0, 0, 180))
        let t1w: float = game.text_width("Game over", 30)
        game.draw_text("Game over", (W - t1w) / 2.0, 250.0, 30, game.RED)
        let t2 = f"score {score}   best {best}"
        let t2w: float = game.text_width(t2, 20)
        game.draw_text(t2, (W - t2w) / 2.0, 292.0, 20, game.WHITE)
        let t3w: float = game.text_width("Space to restart", 18)
        game.draw_text("Space to restart", (W - t3w) / 2.0, 318.0, 18, game.WHITE)
    end
```

E o placar centrado de verdade (no `draw`, substitui o draw_text do score):

```noxy
    let sw: float = game.text_width(f"{score}", 40)
    game.draw_text(f"{score}", (W - sw) / 2.0, 24.0, 40, game.WHITE)
```

- [ ] **Step 5: Rodar os três exemplos**

Run (cada um; janela abre — fechar com Escape, e no flappy morrer uma vez
para ouvir hit e ver o painel):
```bash
cd /c/Users/sandr/Documents/noxy && ./noxy.exe ../noxy_game_engine/examples/sprite.nx
cd /c/Users/sandr/Documents/noxy && ./noxy.exe ../noxy_game_engine/examples/animation.nx
cd /c/Users/sandr/Documents/noxy && ./noxy.exe ../noxy_game_engine/examples/flappy_bird.nx
```
Expected: sprite mostra a cópia translúcida espelhada no mouse; animation
anda e vira; flappy toca flap/score/hit, painel translúcido, textos
centrados. Para validar sem interação, usar a técnica do MVP: cópia
descartável do flappy no scratchpad com autoplay via sed
(`started = true`, flap aleatório, auto-restart) rodando ~10 s.

- [ ] **Step 6: Commit**

```bash
git add examples/sprite.nx examples/animation.nx examples/walker.png examples/flappy_bird.nx examples/flap.wav examples/score.wav examples/hit.wav
git commit -m "docs(examples): animation com sprite sheet; flappy com som, painel translucido e texto centrado"
```

---

### Task 8: README, issue e push

**Files:**
- Modify: `README.md`
- Modify: issue #1 (via `gh`)

**Interfaces:**
- Consumes: tudo acima.
- Produces: docs publicadas; branch pronta para a tag v0.2.0 (que só sai
  com confirmação do usuário).

- [ ] **Step 1: Atualizar o README**

Mudanças, seção a seção:
- Instalação: exemplo vira `@v0.2.0`.
- Tabela de desenho: `draw_image_ex` com a assinatura nova; linhas novas
  para `draw_image_sub(img, sx, sy, sw, sh, x, y)` e `draw_image_sub_ex(img,
  sx, sy, sw, sh, x, y, scale, angle_deg, opacity, flip_x)`; nota de que
  `opacity` é 0..1 e `flip_x` espelha horizontalmente antes da rotação.
- Cores: `Color(r, g, b, a)`, `rgb()` (alfa 255), `rgba(r, g, b, a)`;
  constantes opacas; alfa vale para todas as primitivas e para o texto.
- Seção nova **Audio** (entre Drawing e Input) com a tabela:
  `load_sound(path) -> Sound` (WAV/OGG inteiro na memória),
  `play(s: Sound)` (sobreposição livre), `play_music(path)` (streaming em
  loop; trocar música = chamar de novo), `stop_music()`,
  `set_volume(v: float)` (global 0..1; música na hora, sons no próximo
  play). Nota: áudio não exige `init`; formatos WAV e OGG Vorbis detectados
  pelo conteúdo; música OGG recomendada (não há asset de exemplo no repo).
- `text_width(s, size) -> float` na tabela de desenho (com a dica: num laço
  quente, medir uma vez e guardar).
- Lista de exemplos: `animation.nx` (sprite sheet + flip) e a menção de que
  o flappy agora tem som.
- Performance notes: uma frase de que áudio e `text_width` são chamadas
  avulsas fora do lote do `flip`.

- [ ] **Step 2: Verificação final completa**

Run: `go build ./... && go vet ./... && go test -race ./... -count=1`
Expected: PASS. Conferir `git status` — nada de binário ou arquivo do
scratchpad no stage.

- [ ] **Step 3: Commit e push**

```bash
git add README.md
git commit -m "docs(readme): API v0.2 — alfa, sprite sheet, audio e text_width"
git push
```

- [ ] **Step 4: Atualizar a issue #1**

Marcar os 4 checkboxes da seção v0.2 como feitos: baixar o corpo com
`gh issue view 1 --json body -q .body` para um arquivo no scratchpad,
trocar `- [ ]` por `- [x]` só nas 4 linhas da seção "v0.2", e
`gh issue edit 1 --body-file <arquivo>`. Comentar na issue:
`gh issue comment 1 --body "v0.2 implementado em main (áudio WAV/OGG, draw_image_sub/_ex, rgba/opacity/flip_x, text_width). Sai no release v0.2.0."`

- [ ] **Step 5: Perguntar ao usuário sobre a tag**

Reportar o estado e perguntar se cria a tag `v0.2.0` (dispara o workflow de
release com os 4 binários). NÃO criar a tag sem resposta explícita.
