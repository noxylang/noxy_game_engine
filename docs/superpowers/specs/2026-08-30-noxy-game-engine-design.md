# noxy_game_engine — design (MVP)

Data: 2026-08-30. Estado: aprovado em conversa (backend Ebiten, nome `game`,
sem áudio no MVP).

## Objetivo

Uma engine de jogos 2D para Noxy no espírito do pygame: simples, imediata,
prática. Publicada como **extensão por processo** (`kind = "process"`), no
mesmo formato de `github.com/estevaofon/noxy_dynamodb`: `noxy --get` baixa o
binário da plataforma, sem toolchain no lado do usuário.

Fora do MVP: áudio, sprites animados, câmera, colisões, fontes customizadas,
fullscreen, gamepad. Nada disso é impedido pelo design; entram como exports
novos.

## Uso alvo

```noxy
use github_com.estevaofon.noxy_game_engine as game

game.init(640, 480, "Pong")
let x = 100.0
while game.running() do
    if game.key_down("right") then x = x + 4.0 end
    if game.key_pressed("escape") then game.stop() end
    game.clear(game.BLACK)
    game.draw_rect(x, 200.0, 40.0, 40.0, game.RED)
    game.draw_circle(320.0, 240.0, 10.0, game.WHITE)
    game.draw_text("score: 3", 10.0, 10.0, 16, game.WHITE)
    game.flip()
end
game.quit()
```

`flip()` é o `pygame.display.flip()` + `clock.tick(60)` + `event.get()` numa
chamada só: envia o frame, espera o próximo tick do Ebiten (60 Hz) e atualiza
o snapshot de input.

## Arquitetura

```
script.nx ── use ──► noxy_game_engine.nx (wrapper)  ── 1 chamada/frame ──► noxy-plugin-game (Go + Ebiten)
                     acumula comandos em estado         game_flip(cmds)         goroutine SDK ⇄ loop Ebiten
                     de módulo; guarda o snapshot   ◄── snapshot de input ──    (thread principal)
```

Motivação: uma chamada ao processo custa ≈45 µs (docs/EXTENSIONS.md do Noxy).
Uma chamada por desenho não escala; um frame por chamada custa o mesmo que um
desenho.

### Componentes

| Peça | Responsabilidade |
|---|---|
| `noxy_ext.toml` | `name = "game"`, `kind = "process"`, `concurrency = "single"`, exports abaixo, `[binaries]` por plataforma |
| `noxy_game_engine.nx` | API pública tipada; estado de módulo (`_cmds`, snapshot de input, `_running`); constantes de cor |
| `main.go` | `main()`: `go p.Main()`; a thread principal espera o pedido de `init` e chama `ebiten.RunGame` |
| `engine.go` | `engine`: estado compartilhado entre a goroutine do SDK e o loop Ebiten — frame pendente, snapshot de input, imagens por handle, flags `closed`/`stopped` |
| `commands.go` | decodificação de `[]any` → `[]command` tipados; **sem dependência de Ebiten** (testável headless) |
| `keys.go` | tabela `ebiten.Key` → nome Noxy (`"left"`, `"space"`, `"a"`, `"0"`...) |
| `render.go` | `Draw(screen)`: executa a lista de comandos com `vector`, `text/v2` (Go Regular embutida) e `DrawImage` |
| `examples/*.nx` | `smoke.nx`, `bouncing_ball.nx`, `pong.nx` |
| `release/build.sh`, `.github/workflows/release.yml` | build por plataforma (ver Plataformas) |

### Exports (`noxy_ext.toml`)

| export | params | returns | notas |
|---|---|---|---|
| `game_init` | `int, int, string` | `void` | abre a janela; retorna após o primeiro `Update`. Segunda chamada é erro |
| `game_flip` | `any[]` | `map[string]any` | entrega o frame, bloqueia até o próximo tick, devolve o snapshot. `timeout_ms = 10000` |
| `game_load_image` | `string` | `map[string]any` | `{"id": int, "width": int, "height": int}`; `stateful = true`; caminho relativo ao cwd do script (o host não muda o cwd do filho) |
| `game_quit` | — | `void` | encerra o Ebiten; a janela fecha. Idempotente |

Toda chamada antes de `game_init` falha com `game not initialized: call
game.init(width, height, title) first`. Depois de `game_quit`, `game_flip`
falha com `game is closed`.

### Protocolo de frame (`cmds: any[]`)

Cada comando é um `any[]` cujo primeiro elemento é a tag. Números aceitam
`int` ou `float` (o decodificador Go aceita `int64` e `float64`). Cores são
três ints 0–255.

| tag | forma | wrapper |
|---|---|---|
| `clear` | `["clear", r, g, b]` | `clear(c: Color)` |
| `rect` | `["rect", x, y, w, h, r, g, b, thickness]` (`thickness` 0 = preenchido) | `draw_rect(x, y, w, h, c)`, `draw_rect_outline(x, y, w, h, c, thickness)` |
| `circle` | `["circle", cx, cy, radius, r, g, b, thickness]` | `draw_circle(cx, cy, radius, c)`, `draw_circle_outline(cx, cy, radius, c, thickness)` |
| `line` | `["line", x1, y1, x2, y2, r, g, b, thickness]` | `draw_line(x1, y1, x2, y2, c, thickness)` |
| `text` | `["text", s, x, y, size, r, g, b]` | `draw_text(s, x, y, size, c)` — fonte Go Regular embutida; `size` em px |
| `image` | `["image", id, x, y, scale, angle_deg]` | `draw_image(img, x, y)`, `draw_image_ex(img, x, y, scale, angle_deg)` — `x, y` = canto superior esquerdo; rotação em torno do centro da imagem já escalada |

Tag desconhecida, aridade errada ou tipo errado → `extension 'game' failed:
command N: ...`, sem desenhar o frame (o frame anterior fica na tela). O
decodificador valida o frame inteiro antes de entregá-lo ao Ebiten.

O `screen` do Ebiten é limpo a cada `Draw`, então o motor **guarda a última
lista de comandos** e a redesenha a cada `Draw` até receber outra. Um frame
sem `clear` desenha sobre o fundo preto padrão. Consequência: o script decide
o ritmo; se ele demora 100 ms entre `flip`s, a janela continua respondendo (o
loop do Ebiten segue vivo, redesenhando a última lista) e o próximo `flip`
volta no tick seguinte.

### Snapshot de input (retorno de `game_flip`)

```
{
  "closed": bool,          // usuário clicou no X da janela (ou quit)
  "dt": float,             // segundos desde o flip anterior
  "keys_down": string[],   // teclas seguradas neste tick
  "keys_pressed": string[],// teclas que desceram neste tick
  "mouse_x": int, "mouse_y": int,
  "mouse_down": string[],  // "left" | "right" | "middle" seguradas
  "mouse_pressed": string[]
}
```

O wrapper guarda o snapshot em estado de módulo; `key_down(k)`,
`key_pressed(k)`, `mouse_pos() -> Point`, `mouse_down(b)`, `mouse_pressed(b)`,
`delta()` e `running()` só leem esse estado — nenhuma chamada ao processo.

Nomes de tecla (tabela em `keys.go`, documentada no README): letras
minúsculas (`"a"`), dígitos (`"0"`), `"left" "right" "up" "down"`, `"space"
"enter" "escape" "tab" "backspace"`, `"shift" "ctrl" "alt"` (os dois lados
mapeiam para o mesmo nome), `"f1"`..`"f12"`. Tecla fora da tabela usa
`ebiten.Key.String()` em minúsculas.

### Ciclo de vida

1. `game.init` → `game_init` envia `{w, h, title}` num canal; a thread
   principal (que estava bloqueada esperando) configura a janela e chama
   `ebiten.RunGame`. `game_init` retorna quando o primeiro `Update` sinaliza
   que a janela existe. `ebiten.SetWindowClosingHandled(true)`: o X não
   encerra o processo, só marca `closed` (visível no próximo snapshot →
   `running()` vira `false`).
2. `game.flip` → publica o frame, espera o `Update` seguinte consumi-lo e
   devolve o snapshot colhido nesse `Update`.
3. `game.stop()` (wrapper) só faz `_running = false` — sai do laço.
4. `game.quit` → `Update` retorna `ebiten.Termination`; `RunGame` volta; a
   thread principal fica bloqueada num `select {}` até o host fechar o stdin
   (o `p.Main()` do SDK então dá `os.Exit`). Sem isso o processo morreria com
   a janela e o host reportaria `trapped`.
5. Script termina sem `quit`: o host fecha o stdin, o SDK sai, a janela some
   com o processo.

### Wrapper (`noxy_game_engine.nx`)

- `struct Color { r: int, g: int, b: int }`, `struct Image { id: int, width: int, height: int }`,
  `struct Point { x: int, y: int }`.
- Constantes: `BLACK WHITE RED GREEN BLUE YELLOW` e `rgb(r, g, b) -> Color`.
- `init(width: int, height: int, title: string) -> void` — chama `game_init`,
  põe `_running = true`, zera o snapshot.
- Funções de desenho fazem `append(ref _cmds, [...])`.
- `flip() -> void` — `let snap = game_flip(_cmds)`, `_cmds = []`, atualiza
  o estado; `_running = _running && !closed`.
- `load_image(path: string) -> Image` — raise via extensão se o arquivo não
  existe/ não é PNG.
- `quit() -> void` — `_running = false`; `game_quit()`.

Erros da extensão são erros de runtime (`extension 'game' failed: ...`),
capturáveis com `call_result` — nada devolve `null` silenciosamente.

## Plataformas e release

Ebiten é Go puro no Windows; em Linux precisa de cgo + X11/GL headers, em
macOS de cgo. O `release/build.sh` do SDK (`CGO_ENABLED=0`, cross-compile)
**não serve**. O workflow de release usa um job por SO com o runner nativo:
`windows-latest` (amd64), `ubuntu-latest` (amd64, após `apt-get install
libgl1-mesa-dev xorg-dev libasound2-dev`), `macos-latest` (arm64 e amd64 com
`GOARCH`). `[binaries]` lista só o que o workflow produz: `windows-amd64`,
`linux-amd64`, `darwin-amd64`, `darwin-arm64`. `release/build.sh` compila só
a plataforma corrente e escreve `dist/checksums.txt` — o job final agrega os
checksums.

Desenvolvimento local (Windows): `go build -o bin/noxy-plugin-game-windows-amd64.exe .`
e o checkout copiado/linkado em `<projeto>/noxy_libs/github_com/estevaofon/noxy_game_engine`.

## Testes

- Go, headless: `commands_test.go` (decodificação de cada tag, erros por
  tag/aridade/tipo, ints e floats), `keys_test.go` (tabela), `engine_test.go`
  (troca de frame e snapshot sem Ebiten: `engine.submit(frame)` +
  `engine.tick(input)` simulando `Update`; `closed`/`quit`; erro antes de
  `init`).
- Noxy: `examples/` rodam à mão (janela). `examples/smoke.nx` abre a janela,
  desenha 3 frames e faz `quit` — serve para verificar a instalação, e é o
  teste de integração manual do binário.
- Verificação: `go build ./... && go vet ./... && go test ./...`, depois
  `noxy examples/smoke.nx` a partir de um projeto com o pacote instalado.

## Estrutura do repositório

```
noxy_game_engine/
├── .github/workflows/release.yml
├── .gitignore                 # bin/ dist/
├── go.mod / go.sum
├── main.go engine.go commands.go keys.go render.go font.go
├── *_test.go
├── noxy.mod                   # module noxy_game_engine / noxy v0.23.0
├── noxy_ext.toml
├── noxy_game_engine.nx
├── examples/ smoke.nx bouncing_ball.nx pong.nx
├── release/build.sh
├── docs/superpowers/specs/ (este arquivo) e plans/
└── README.md
```
