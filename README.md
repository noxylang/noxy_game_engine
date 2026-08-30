# Noxy Game Engine

A small 2D game engine for the [Noxy](https://github.com/estevaofon/noxy)
language in the spirit of pygame — simple and immediate — shipped as a
**process extension**: `noxy --get` downloads a prebuilt binary for your
platform, verifies it, and records its hash in `noxy.sum`. No Go toolchain,
no build step.

Under the hood the binary is a Go program on [Ebitengine](https://ebitengine.org/);
the Noxy side is a thin typed wrapper. Requires Noxy **0.23.0** or newer.
Binaries are published for windows/amd64, linux/amd64, darwin/amd64 and
darwin/arm64.

## Installation

```bash
noxy --get github.com/estevaofon/noxy_game_engine@v0.1.0
```

Without `@version`, `--get` resolves the newest release tag. The package lands
in `noxy_libs/github_com/estevaofon/noxy_game_engine/` with the binary for
your OS/arch in `bin/`; `noxy.sum` gets one line for the manifest plus one per
published binary, so commit it.

## Usage

```noxy
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
    let dt: float = game.delta()
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

The loop is the whole model: draw calls only queue commands; `flip()` sends
the frame to the window in **one** call, waits for the next tick (60 Hz, so
`flip` also paces the game like pygame's `clock.tick(60)`) and refreshes the
input snapshot that `key_down`, `mouse_pos`, `running` and friends read.
More in `examples/`: `smoke.nx` (installation check), `bouncing_ball.nx`,
`pong.nx`.

## API

### Window and loop

| Function | Description |
|---|---|
| `init(width: int, height: int, title: string)` | Opens the window. Raises if called twice or if the window cannot be opened |
| `running() -> bool` | `true` from `init` until the window is closed, `stop()` or `quit()` |
| `stop()` | Leaves the loop (`running()` becomes `false`); the window stays until `quit()` |
| `quit()` | Closes the window. Idempotent |
| `flip()` | Shows the frame built since the last `flip`, waits for the next tick, refreshes input. Raises on an invalid frame |
| `delta() -> float` | Seconds between the last two flips (≈ 0.0167 at 60 Hz) |

Closing the window with its X button does **not** kill the program: the next
`flip` makes `running()` return `false` and the script decides what to do.

### Drawing (queued until `flip`)

Coordinates are `float` pixels from the top-left corner; an `int` literal is
accepted where a `float` is expected (`draw_rect(10, 20, 30, 40, c)`).

| Function | Description |
|---|---|
| `clear(c: Color)` | Fills the window. Without it a frame draws over black |
| `draw_rect(x, y, w, h, c: Color)` | Filled rectangle |
| `draw_rect_outline(x, y, w, h, c: Color, thickness: float)` | Rectangle outline |
| `draw_circle(cx, cy, radius, c: Color)` | Filled circle |
| `draw_circle_outline(cx, cy, radius, c: Color, thickness: float)` | Circle outline |
| `draw_line(x1, y1, x2, y2, c: Color, thickness: float)` | Line segment |
| `draw_text(s: string, x, y, size: int, c: Color)` | Text with its top-left corner at `(x, y)`, `size` px high, built-in Go Regular font |
| `load_image(path: string) -> Image` | Decodes a PNG or JPEG, relative to the working directory. Raises if missing or not an image |
| `draw_image(img: Image, x, y)` | Draws the image with its top-left corner at `(x, y)` |
| `draw_image_ex(img: Image, x, y, scale: float, angle_deg: float)` | Scaled, then rotated around the center of the scaled image |

Colors: `Color(r, g, b)` (0–255), `rgb(r, g, b)`, and the constants `BLACK`,
`WHITE`, `RED`, `GREEN`, `BLUE`, `YELLOW`. `Image` has `id`, `width`,
`height`.

### Input (snapshot taken by the last `flip`)

| Function | Description |
|---|---|
| `key_down(key: string) -> bool` | The key is held |
| `key_pressed(key: string) -> bool` | The key went down during the last frame |
| `mouse_pos() -> Point` | Cursor position (`x`, `y` ints) |
| `mouse_down(button: string) -> bool` | `"left"`, `"right"` or `"middle"` is held |
| `mouse_pressed(button: string) -> bool` | The button went down during the last frame |

Key names: `"a"`..`"z"`, `"0"`..`"9"`, `"left"` `"right"` `"up"` `"down"`,
`"space"` `"enter"` `"escape"` `"tab"` `"backspace"`, `"shift"` `"ctrl"`
`"alt"` (either side), `"f1"`..`"f12"`, `"numpad0"`..`"numpad9"`. Any other
key is Ebitengine's name in lower case (`"home"`, `"pageup"`, `"comma"`...).

### Errors

A failure inside the extension is a runtime error,
`extension 'game' failed: <message>`, with the Noxy stack of the call site —
for example `load_image` on a missing file, or `init` called twice. It can
be captured with `call_result`:

```noxy
use errors select *

let r = call_result(game.load_image, "sprites/hero.png")
if r.ok then
    print(f"{r.value.width}x{r.value.height}")
else
    print(f"could not load: {r.failure.message}")
end
```

If the plugin process dies, the next call fails with
`extension 'game' trapped: ...` and the extension stays unusable for the rest
of the program. `flip` has a 10 s deadline (`extension 'game' timed out`) — it
normally returns in one tick.

### Performance notes

One process call costs ≈45 µs, which is why the whole frame travels in a
single `flip`: a frame with a few hundred draw calls is still well under a
millisecond of transport. The Ebitengine loop keeps running between flips, so
a slow script frame never freezes the window — it just shows the last frame
until the next `flip`.

## Platforms

Ebitengine is pure Go on Windows but needs cgo (and X11/OpenGL libraries) on
Linux and macOS, so binaries are built natively per OS by the release
workflow — there is no `CGO_ENABLED=0` cross-compile. Linux users need a
desktop with X11 or Wayland (XWayland) and an OpenGL driver at run time.
Platforms not listed under `[binaries]` in `noxy_ext.toml` are an error at
`noxy --get` time.

## Development

The extension is a Go program on the Noxy plugin SDK
(`github.com/estevaofon/noxy/sdk/noxyplugin`). To run a checkout without a
release, build your platform's asset (the name is in `[binaries]` of
`noxy_ext.toml`) and point a project at the checkout:

```bash
go test ./...
go build -o bin/noxy-plugin-game-windows-amd64.exe .   # or -linux-amd64, -darwin-arm64, ...
```

Copy (or link) the checkout to
`<project>/noxy_libs/github_com/estevaofon/noxy_game_engine`; without a
`noxy.sum` entry the VM prints a trust-on-first-use warning and runs it.
Then `noxy examples/smoke.nx` should open a window for half a second and
print `ok`.

The Go tests (`commands_test.go`, `keys_test.go`, `engine_test.go`) run
without a window: the frame protocol, the key table and the engine's
frame/snapshot/lifecycle logic are separated from the Ebitengine render.
Design notes live in `docs/superpowers/specs/`.

Releasing: push a tag `vX.Y.Z`. The GitHub Actions workflow
(`.github/workflows/release.yml`) builds each platform on its own runner
with `release/build.sh game`, merges the checksums into `checksums.txt`, and
publishes everything as release assets — the exact layout `noxy --get`
expects.
