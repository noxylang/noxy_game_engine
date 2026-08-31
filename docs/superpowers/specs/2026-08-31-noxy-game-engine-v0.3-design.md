# noxy_game_engine — design v0.3 (fonte, janela, input, polígonos, câmera)

Data: 2026-08-31. Estado: aprovado em conversa (gamepad incluído sem
validação por hardware; fonte via `set_font` global; câmera no wrapper com
`camera_reset` para UI). Implementa o pacote v0.3 da issue #1. Base: specs do
MVP (`2026-08-30-...`) e do v0.2 (`2026-08-31-...-v0.2-design.md`).

## Objetivo

Fechar a segunda leva da issue #1: fonte customizada, controle de janela
(fullscreen e taxa de quadros), o input que faltava (roda do mouse, texto
digitado, gamepad), polígonos preenchidos, e os utilitários que cabem só no
wrapper (câmera e colisão).

Fora do v0.3: timers (cortados — `t = t + dt` num laço imediato já resolve, e
uma API em volta só adiciona superfície), resolução lógica explícita (o
`Layout` já escala e faz letterbox no fullscreen), vibração de gamepad,
mapeamento de controles não-padrão, e MP3.

## Compatibilidade

v0.3 é **aditivo no nível do wrapper**: nenhuma assinatura pública muda.
Internamente o protocolo muda (o comando `text` ganha o id da fonte, e
`game_text_width` ganha o parâmetro da fonte); como wrapper e binário saem
juntos no mesmo release, isso só afetaria quem misturasse versões — o que
`noxy --get` não faz.

## Fonte customizada

`font.go` deixa de ter uma fonte só e vira um registro:

- `fonts map[int64]*text.GoTextFaceSource`, com **id 0 = Go Regular
  embutida** (criada preguiçosamente, como hoje).
- cache de faces por par `(fontID, size)` — chave `faceKey{id, size}`.
- O mutex existente passa a cobrir registro, cache e uso das faces (o
  `GoTextFaceSource` não é seguro para uso concorrente; ver v0.2).

Export novo `game_load_font(path) -> {"id": int}` (`stateful = true`): lê o
arquivo, `text.NewGoTextFaceSource` (aceita TTF e OTF), registra e devolve o
id. Arquivo ausente ou que não é fonte vira erro da extensão. Não exige
`game_init`.

`game_text_width` passa a `(s, size, font_id) -> float`; id desconhecido é
erro. O comando `text` do frame ganha o id no fim:
`["text", s, x, y, size, r, g, b, a, font_id]` (aridade 10). Id desconhecido
num frame é validado junto com os ids de imagem, no `handleFlip`, com a
mesma forma de mensagem: `command N: unknown font 3 (not returned by
load_font)`.

Wrapper: `struct Font { id: int }`, `load_font(path) -> Font`, `set_font(f)`,
`reset_font()` e estado `_font: int` (0). `draw_text` e `text_width` mantêm
a assinatura e usam `_font`.

## Janela

Dois exports novos, ambos exigindo `game_init` (são operações de janela):

| export | params | returns |
|---|---|---|
| `game_set_fullscreen` | `bool` | `void` |
| `game_set_fps` | `int` | `void` |

Aplicação: o handler guarda o pedido no engine (`pendingFullscreen *bool`,
`pendingTPS *int`) e o `tick` aplica na thread principal, chamando
`ebiten.SetFullscreen` / `ebiten.SetTPS`. Mesmo padrão do `init` — não
depende de garantias de concorrência do Ebiten. O efeito aparece no tick
seguinte, o que é invisível para quem chama entre flips.

`game_set_fps` valida `n >= 1`. Escala e letterbox no fullscreen saem de
graça do `Layout`, que continua devolvendo o tamanho lógico fixo.

Wrapper: `set_fullscreen(on: bool)`, `set_fps(n: int)`.

## Input novo

O snapshot de `game_flip` ganha:

```
"wheel_x": float, "wheel_y": float   // ebiten.Wheel() neste tick
"text_input": string                 // caracteres digitados neste tick
"pad_down": string[]                 // "0:a", "1:start", ...
"pad_pressed": string[]              // idem, só o que desceu neste tick
"pad_axes": float[]                  // 6 valores por controle conectado
```

Numeração de controles: **posição na lista de ids conectados** (ordenada
pelo Ebiten), não o id bruto — o primeiro controle é sempre `0`. Sem
controles, `pad_down`/`pad_pressed` vêm vazios e `pad_axes` vem vazio.

Prefixo em vez de aninhamento: `pad_down` é um `string[]` plano com o número
do controle no nome (`"0:a"`). Alternativa rejeitada: `any[]` de `string[]`
(um por controle) — mais bonito, mas estreia no wrapper uma conversão de
tipo que ele ainda não exercita. O prefixo reusa o `_has` que já existe.

Nomes de botão (layout padrão do Ebiten, `StandardGamepadButton`):

| nome | botão |
|---|---|
| `a` `b` `x` `y` | cluster direito: bottom, right, left, top |
| `up` `down` `left` `right` | direcional |
| `start` `back` `guide` | centro |
| `lb` `rb` | ombros (front top left/right) |
| `lt` `rt` | gatilhos (front bottom left/right), também como botão |
| `lstick` `rstick` | cliques dos analógicos |

Eixos (6 por controle, nesta ordem em `pad_axes`): `left_x`, `left_y`,
`right_x`, `right_y`, `lt`, `rt`. Os dois últimos são o valor analógico dos
gatilhos (`StandardGamepadButtonValue`), 0..1.

Controle sem layout padrão disponível (`IsStandardGamepadLayoutAvailable`
falso) é ignorado — não entra na contagem nem nos arrays. Documentado no
README.

> **Revisto na v0.3.1.** Essa regra estava errada na prática: controles
> genéricos USB (o GUID `03000000790000000600000000000000`, comum) não têm
> layout padrão na base do Ebiten e sumiam por completo. Desde a v0.3.1 eles
> entram com nomes brutos (`"b0"`.., `"a0"`..), o snapshot passou a carregar
> `pad_count`, `pad_names`, `pad_kinds`, `pad_axis_names` e
> `pad_axis_values` (nomes e valores paralelos, porque a contagem de eixos
> varia por dispositivo), e existe `add_gamepad_mapping` para carregar uma
> linha do SDL_GameControllerDB.

Wrapper: `wheel_x() -> float`, `wheel_y() -> float`, `text_input() ->
string`, `gamepad_count() -> int`, `gamepad_down(pad: int, button: string)
-> bool`, `gamepad_pressed(pad: int, button: string) -> bool`,
`gamepad_axis(pad: int, axis: string) -> float` (eixo desconhecido ou
controle fora da faixa devolve `0.0`).

`inputReader` (a interface que o `tick` lê) ganha `wheel() (float64,
float64)`, `textInput() string` e `pads() (down, pressed []string, axes
[]float64)`; a implementação sobre o Ebiten fica em `render.go` e a fake dos
testes em `engine_test.go`.

## Polígonos

Comando novo, aridade 7:

```
["polygon", [x1, y1, x2, y2, ...], r, g, b, a, thickness]
```

A lista de pontos é um `any[]` aninhado no elemento 1 — assim a aridade
segue fixa, como o decodificador assume. Validação: elemento 1 é array, a
quantidade de números é par, e há **no mínimo 3 pontos** (6 números);
`thickness` 0 = preenchido, como nas outras primitivas. Mensagens no padrão
existente: `command 0: element 1 of "polygon": needs at least 3 points, got
2`.

`command` ganha `points []float64`. Render: `vector.Path` com `MoveTo` +
`LineTo` + `Close`, depois `FillPath` (thickness 0) ou `StrokePath`.

Wrapper: `draw_polygon(points: float[], c: Color)` e
`draw_polygon_outline(points: float[], c: Color, thickness: float)` — os
pontos vão como pares planos `[x1, y1, x2, y2, ...]`.

## Câmera e colisão (só wrapper)

Nada disso toca no binário: é Noxy puro no `noxy_game_engine.nx`.

**Câmera:** estado `_cam_x`, `_cam_y` (0 por padrão); `camera_set(x, y)` e
`camera_reset()`. Todo `draw_*` subtrai o offset ao enfileirar — `rect`,
`circle`, `line`, `text`, `image` (todas as variantes) e `polygon`. `clear`
não é afetado. Padrão documentado no README: desenhe o mundo, chame
`camera_reset()`, desenhe a UI.

`mouse_pos()` continua em coordenadas de tela; quem precisa da posição no
mundo soma a câmera. Para isso o wrapper expõe `camera_pos() -> Point`.

**Colisão:** `rect_overlaps(ax, ay, aw, ah, bx, by, bw, bh) -> bool`,
`circle_overlaps(ax, ay, ar, bx, by, br) -> bool`,
`circle_rect_overlaps(cx, cy, r, x, y, w, h) -> bool`,
`point_in_rect(px, py, x, y, w, h) -> bool`.

## Testes

- `font_test.go`: id 0 é a embutida; `loadFont` de arquivo ausente e de
  arquivo que não é fonte falham; ids sobem 1, 2, ...; `textWidth` com id
  desconhecido dá erro; cache por par (fonte, tamanho); concorrência com
  `-race` (o teste existente ganha o registro novo).
- `commands_test.go`: `polygon` válido (preenchido e contorno), elemento 1
  não-array, contagem ímpar, menos de 3 pontos, `text` com aridade 10 e id
  de fonte.
- `engine_test.go`: `pendingFullscreen`/`pendingTPS` aplicados pelo `tick`;
  `set_fullscreen`/`set_fps` antes do `init` falham; `set_fps(0)` falha;
  frame com id de fonte desconhecido é rejeitado; snapshot novo (roda, texto
  digitado, pads) via `fakeInput`.
- `pads_test.go` (novo): tabela de nomes de botão e eixo, e a montagem
  `"0:a"` a partir de uma fonte de gamepad fake (a leitura do Ebiten fica
  atrás de uma função pequena, testada por tabela — não há controle no CI).
- Manual: `examples/showcase.nx` novo (roda, texto digitado, polígono,
  câmera, fullscreen) e autoplay dos exemplos existentes.

## Exemplos e docs

- `examples/showcase.nx` (novo): um polígono que segue a câmera, zoom pela
  roda do mouse, uma caixa de texto que mostra `text_input()`, `F` alterna
  fullscreen, `Escape` sai. Serve de demo dos itens do v0.3 que dá para
  exercitar sem hardware extra.
- `load_font` fica documentado no README, **sem asset no repositório**
  (licença de fonte): o exemplo usa a embutida. A validação de ponta a ponta
  é local, com uma fonte do sistema, num script descartável.
- README: seções novas (fonte, janela, gamepad, polígono, câmera, colisão),
  nota sobre controles sem layout padrão, e o padrão mundo→UI da câmera.
- Issue #1: marcar os itens do v0.3 e os de "só wrapper" que entraram
  (colisão e câmera; timers ficam desmarcados com nota de que foram
  cortados).

## Release

Sem mudança de infraestrutura (mesmos 4 alvos, mesmo workflow). Ao final:
commits, push, e tag **v0.3.0** com confirmação do usuário, seguida da
validação do `noxy --get ...@v0.3.0` num projeto limpo.
