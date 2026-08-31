# noxy_game_engine — design v0.2 (áudio, sprite sheet, alfa, text_width)

Data: 2026-08-31. Estado: aprovado em conversa (pode quebrar API no 0.x;
áudio mínimo + volume global; formatos WAV e OGG). Implementa o pacote v0.2
da issue #1. Base: spec do MVP em `2026-08-30-noxy-game-engine-design.md`.

## Objetivo

Fechar as quatro lacunas que separam "demo" de "jogo": efeitos sonoros e
música, animação por sprite sheet, transparência (alfa e opacity), e medida
de texto para centralizar UI.

Fora do v0.2 (ficam na issue #1): volume por som, pause/resume de música,
`is_playing`, MP3, fontes customizadas, fullscreen, `set_fps`, gamepad,
roda do mouse, eventos de caractere, polígonos, e os itens "só wrapper"
(colisão, câmera, timers).

## Quebras de compatibilidade (decididas)

v0.2 é 0.x: quebra permitida em troca de API limpa.

1. `struct Color` ganha `a: int` (0–255). `rgb(r, g, b)` retorna `a = 255`;
   quem constrói `Color(r, g, b)` direto passa a precisar do quarto campo.
   Constantes (`BLACK`, `WHITE`, ...) atualizadas com `a = 255`.
2. `draw_image_ex` vira `(img, x, y, scale, angle_deg, opacity: float,
   flip_x: bool)`.
3. Protocolo de frame: toda cor vira 4 componentes; o comando `image` é
   unificado com sub-retângulo + opacity + flip_x (aridades abaixo). O
   protocolo é interno (wrapper e plugin saem juntos no mesmo release),
   então só afeta quem misturar versões — o que `noxy --get` não faz.

## Protocolo de frame v2

Cores passam a `r, g, b, a` (ints 0–255; `a = 255` opaco). Aridades novas:

| tag | forma | wrapper |
|---|---|---|
| `clear` | `["clear", r, g, b, a]` (5) | `clear(c)` |
| `rect` | `["rect", x, y, w, h, r, g, b, a, thickness]` (10) | `draw_rect`, `draw_rect_outline` |
| `circle` | `["circle", cx, cy, radius, r, g, b, a, thickness]` (9) | `draw_circle`, `draw_circle_outline` |
| `line` | `["line", x1, y1, x2, y2, r, g, b, a, thickness]` (10) | `draw_line` |
| `text` | `["text", s, x, y, size, r, g, b, a]` (9) | `draw_text` |
| `image` | `["image", id, sx, sy, sw, sh, x, y, scale, angle_deg, opacity, flip_x]` (12) | `draw_image`, `draw_image_ex`, `draw_image_sub`, `draw_image_sub_ex` |

`image` unificado: `sx, sy, sw, sh` é o sub-retângulo de origem em pixels da
imagem (o wrapper manda `0, 0, width, height` quando não há recorte);
`opacity` 0..1; `flip_x` bool espelha horizontalmente. Ordem de aplicação no
render: recorte → flip → escala → rotação em torno do centro do recorte
escalado → translação para `(x, y)` = canto superior esquerdo.

Validações novas (frame inteiro rejeitado antes de desenhar, como no MVP):
alfa fora de 0..255; `opacity` fora de 0..1; `flip_x` não-bool; sub-retângulo
com `sw`/`sh` não positivos. Sub-retângulo fora dos limites da imagem é
validado junto com o id (na fase que já checa ids, pois precisa das
dimensões): `command N: element M of "image": source rect 40,0 32x32 outside
image 2 (64x32)`.

Semântica de alfa no desenho: cor com `a < 255` desenha com blend padrão
(source-over) — `vector.FillRect`/etc. recebem `color.NRGBA`. `opacity` de
imagem usa `ColorScale.ScaleAlpha`.

## Áudio

Arquitetura escolhida: **exports próprios, fora do frame** (som não é
por-frame; ~45 µs por chamada é imperceptível). Alternativas rejeitadas:
comandos no lote do `flip` (atrasa o som em até um frame) e processo de
áudio separado (dobra a infraestrutura de release).

### API do wrapper

| função | comportamento |
|---|---|
| `load_sound(path: string) -> Sound` | Decodifica WAV ou OGG **inteiro para a memória** (PCM). Raise se o arquivo não existe ou não decodifica. `struct Sound { id: int }` |
| `play(s: Sound)` | Toca o som do início, com sobreposição livre (cada play é um player novo, fire-and-forget). Volume = volume global no momento do play |
| `play_music(path: string)` | Toca WAV/OGG **em streaming** do arquivo, em loop infinito. Chamada com música já tocando troca a música (para a anterior). Raise se o arquivo não decodifica |
| `stop_music()` | Para e fecha a música. Idempotente |
| `set_volume(v: float)` | Volume global 0..1 (default 1). Aplica na música imediatamente e nos `play` seguintes; sons já disparados terminam no volume em que começaram. Raise fora de 0..1 |

Formatos: WAV (`audio/wav`) e OGG Vorbis (`audio/vorbis`) — decoders puros-Go
do Ebiten. A extensão do arquivo não importa: tenta-se decodificar por
conteúdo (vorbis primeiro se começa com "OggS", senão WAV).

Efeito na memória: som decodificado vira PCM 16-bit estéreo a 48000 Hz
(`audio.NewContext(48000)`); efeitos de poucos segundos custam < 1 MB.
Música não é decodificada inteira: o arquivo fica aberto com
`audio.NewInfiniteLoop` sobre o stream.

### Exports novos (`noxy_ext.toml`)

| export | params | returns | notas |
|---|---|---|---|
| `game_load_sound` | `string` | `map[string]any` | `{"id": int}`; `stateful = true` |
| `game_play_sound` | `int` | `void` | id desconhecido é raise; `stateful = true` |
| `game_play_music` | `string` | `void` | `stateful = true` |
| `game_stop_music` | — | `void` | `stateful = true` |
| `game_set_volume` | `float` | `void` | `stateful = true` |

Áudio **não exige `game_init`**: o contexto de áudio (`audio.Context`,
singleton do processo) é criado preguiçosamente na primeira chamada de
áudio. Depois de `game_quit` o áudio continua válido (o processo vive até o
host fechar o stdin); `stop_music` no `game_quit` não é feito — quem quer
silêncio chama `stop_music`.

Concorrência: só a goroutine do SDK toca nos players e no registro
(`concurrency = "single"` serializa as chamadas); o mixer do Ebiten roda na
goroutine própria dele, projetado para isso. Nenhum estado de áudio é
compartilhado com o loop de render.

### Estrutura no plugin

`audio.go`: `audioEngine` com `sounds map[int64]*soundEntry`, `nextID`,
`music *musicEntry` (player + closer), `volume float64`, e o
`*audio.Context` preguiçoso. Decodificação separada da reprodução:
`decodeAudio(r io.Reader) (io.ReadSeeker, error)` (headless, testável — não
precisa de contexto) vs. criação de players (precisa do contexto real; os
testes cobrem decodificação, registro, ids e erros, não a saída de som).

## text_width

`text_width(s: string, size: int) -> float` no wrapper → export
`game_text_width(string, int) -> float` (sem `stateful`). Mede com
`text.Advance(s, fontFace(size))` — a métrica vive no plugin (fonte Go
Regular embutida). Não exige `game_init`. É a primeira função chamada fora
do laço quente que retorna dado; uma chamada de processo por medida é
aceitável — o README recomenda medir uma vez e guardar quando estiver num
laço.

`font.go` ganha um `sync.Mutex` no cache de faces: até aqui só a goroutine
de `Draw` tocava o cache; agora a goroutine do SDK (via `game_text_width`)
também.

## Wrapper (`noxy_game_engine.nx`) — resumo das mudanças

- `struct Color { r, g, b, a: int }`; `rgb(r, g, b)` → `a = 255`;
  `rgba(r, g, b, a)`; constantes com `a = 255`.
- `struct Sound { id: int }`.
- Desenho: todos os appends ganham `c.a`; `draw_image(img, x, y)` manda
  `[..., 0, 0, img.width, img.height, x, y, 1.0, 0.0, 1.0, false]`;
  `draw_image_ex(img, x, y, scale, angle_deg, opacity, flip_x)`;
  `draw_image_sub(img, sx, sy, sw, sh, x, y)` (scale 1, sem rotação, opaco);
  `draw_image_sub_ex(img, sx, sy, sw, sh, x, y, scale, angle_deg, opacity,
  flip_x)`.
- Áudio e `text_width` como acima — chamadas diretas, nada passa por `_cmds`.

## Testes

- `commands_test.go`: aridades novas em todos os casos existentes; casos
  novos — alfa fora de 0..255, `opacity` fora de 0..1, `flip_x` não-bool,
  `sw`/`sh` ≤ 0, aridade antiga de `image` (6) agora é erro.
- `engine_test.go`: validação de sub-retângulo contra as dimensões da
  imagem registrada (junto do teste de id inválido existente).
- `audio_test.go` (novo, headless): decodificação de um WAV gerado em
  código no próprio teste (não há encoder OGG puro-Go, então o caminho
  vorbis é testado pela detecção do prefixo `OggS` com payload inválido),
  erro em arquivo inválido, registro de ids, `play` de id desconhecido,
  volume fora de 0..1 — tudo sem criar `audio.Context`.
- `font_test.go` (novo): `textWidth("", n) == 0`, largura cresce com a
  string e com o size, acesso concorrente ao cache (com `-race`).
- Manual: `smoke.nx` continua o teste de instalação; exemplos novos abaixo.

## Exemplos e docs

- `sprite.nx`: ajustado à nova assinatura de `draw_image_ex` (+ demonstra
  opacity e flip_x).
- `examples/animation.nx` (novo): sprite sheet `examples/walker.png`
  (gerado: N frames de um boneco andando lado a lado) animado com
  `draw_image_sub`, virando com `flip_x`.
- `flappy_bird.nx`: efeitos `flap`/`score`/`hit` com WAVs curtos gerados e
  commitados em `examples/` (`load_sound` + `play`); painel de game over
  vira `rgba(0, 0, 0, 180)`; textos centralizados com `text_width`.
- Música OGG: só documentada no README (sem asset de exemplo no repo).
- README: seções de API atualizadas (cores/alfa, imagem, áudio,
  `text_width`), exemplos listados; issue #1 ganha os checks do v0.2.

## Release

Nada muda na infraestrutura (mesmos 4 targets, mesmo workflow): os pacotes
`audio/wav` e `audio/vorbis` são Go puro e as dependências de sistema já
estão nos runners. Ao final: commit(s), push, tag **v0.2.0** (com
confirmação do usuário antes da tag), release via CI, e `noxy --get ...
@v0.2.0` validado num projeto limpo.
