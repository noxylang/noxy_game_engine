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
noxy --get github.com/estevaofon/noxy_game_engine@v0.3.0
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
the frame to the window in **one** call, waits for the next tick (60 Hz by
default — see `set_fps` — so `flip` also paces the game like pygame's
`clock.tick(60)`) and refreshes the
input snapshot that `key_down`, `mouse_pos`, `running` and friends read.
More in `examples/`: `smoke.nx` (installation check), `bouncing_ball.nx`,
`pong.nx`, `sprite.nx` (images: plain, scaled, rotating, and a
half-transparent mirrored copy on the mouse), `animation.nx` (a sprite sheet
walked frame by frame, flipped when it turns around), `flappy_bird.nx` (a
complete little game: gravity, scrolling pipes, score, sound effects, game
over and restart), `showcase.nx` (camera, polygons, mouse wheel, typed text
and fullscreen in one screen).

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
| `draw_text(s: string, x, y, size: int, c: Color)` | Text with its top-left corner at `(x, y)`, `size` px high, in the current font (built-in Go Regular unless you call `set_font`) |
| `text_width(s: string, size: int) -> float` | Width of `s` in px in that same font — for centering. This one *is* a call to the process, so in a hot loop measure once and keep the result |
| `load_image(path: string) -> Image` | Decodes a PNG or JPEG, relative to the working directory. Raises if missing or not an image |
| `draw_image(img: Image, x, y)` | Draws the image with its top-left corner at `(x, y)` |
| `draw_image_ex(img: Image, x, y, scale: float, angle_deg: float, opacity: float, flip_x: bool)` | Mirrored (if `flip_x`), scaled, then rotated around the center of the scaled image; `opacity` 0–1 |
| `draw_image_sub(img: Image, sx, sy, sw, sh, x, y)` | Draws the `(sx, sy, sw, sh)` rectangle of `img` — one frame of a sprite sheet — at `(x, y)` |
| `draw_image_sub_ex(img: Image, sx, sy, sw, sh, x, y, scale, angle_deg, opacity, flip_x)` | A sprite sheet frame with the full transform |
| `draw_polygon(points: float[], c: Color)` | Filled polygon through the flat point list `[x1, y1, x2, y2, ...]` (at least 3 points) |
| `draw_polygon_outline(points: float[], c: Color, thickness: float)` | The outline of that polygon |

Colors: `Color(r, g, b, a)` with components 0–255, `rgb(r, g, b)` (opaque),
`rgba(r, g, b, a)`, and the constants `BLACK`, `WHITE`, `RED`, `GREEN`,
`BLUE`, `YELLOW` (all opaque). Alpha works on every primitive and on text,
so `rgba(0, 0, 0, 180)` is the usual dimming panel; on `clear` it just fills
with the color. `Image` has `id`, `width`, `height`.

A source rectangle outside the image, an `opacity` outside 0–1, a
non-positive `sw`/`sh`, or a polygon with fewer than 3 points is a frame
error, like any other bad command.

### Fonts

| Function | Description |
|---|---|
| `load_font(path: string) -> Font` | Registers a TTF or OTF file. Raises if missing or not a font. Needs no `init` |
| `set_font(f: Font)` | `draw_text` and `text_width` use `f` from here on |
| `reset_font()` | Back to the built-in Go Regular font |

The current font is module state, so `draw_text` and `text_width` keep their
signatures: set the font, draw, reset. No font file ships with the package —
point `load_font` at one you have the rights to.

### Window

| Function | Description |
|---|---|
| `set_fullscreen(on: bool)` | Fullscreen and back. The drawing area keeps the logical size given to `init` and is scaled, with letterboxing |
| `set_fps(n: int)` | How many times per second `flip` returns (default 60, minimum 1) |

Both need `init`, and take effect on the next tick — invisible when you call
them between flips.

### Audio

Sound needs no `init` — it works with or without a window. WAV and OGG
Vorbis are accepted, detected by content rather than by extension.

| Function | Description |
|---|---|
| `load_sound(path: string) -> Sound` | Decodes a whole file into memory (use it for short effects). Raises if missing or not audio |
| `play(s: Sound)` | Plays from the start; sounds overlap freely, and calling it again while it plays layers a second copy |
| `play_music(path: string)` | Streams the file in an endless loop (so a long track costs no memory). Calling it again switches tracks |
| `stop_music()` | Stops and releases the music. Idempotent |
| `set_volume(v: float)` | Global volume 0–1. Applies to the music at once and to the next `play`; sounds already playing keep the volume they started with |

`Sound` has an `id`. Effects decode to PCM at 48 kHz, so a couple of seconds
costs well under a megabyte; music is best kept as OGG.

### Input (snapshot taken by the last `flip`)

| Function | Description |
|---|---|
| `key_down(key: string) -> bool` | The key is held |
| `key_pressed(key: string) -> bool` | The key went down during the last frame |
| `mouse_pos() -> Point` | Cursor position (`x`, `y` ints) |
| `mouse_down(button: string) -> bool` | `"left"`, `"right"` or `"middle"` is held |
| `mouse_pressed(button: string) -> bool` | The button went down during the last frame |
| `wheel_x() -> float`, `wheel_y() -> float` | How far the mouse wheel moved during the last frame (0 when still) |
| `text_input() -> string` | The characters typed during the last frame — for name entry. Empty when nothing was typed |

Key names: `"a"`..`"z"`, `"0"`..`"9"`, `"left"` `"right"` `"up"` `"down"`,
`"space"` `"enter"` `"escape"` `"tab"` `"backspace"`, `"shift"` `"ctrl"`
`"alt"` (either side), `"f1"`..`"f12"`, `"numpad0"`..`"numpad9"`. Any other
key is Ebitengine's name in lower case (`"home"`, `"pageup"`, `"comma"`...).

### Gamepads

| Function | Description |
|---|---|
| `gamepad_count() -> int` | How many controllers are connected. Pads are numbered `0` to `gamepad_count() - 1` |
| `gamepad_name(pad: int) -> string` | What the system calls the controller (`""` for a pad that is not connected) |
| `gamepad_standard(pad: int) -> bool` | Whether this controller has a known layout — which decides the button names below |
| `gamepad_down(pad: int, button: string) -> bool` | The button is held |
| `gamepad_pressed(pad: int, button: string) -> bool` | The button went down during the last frame |
| `gamepad_axis(pad: int, axis: string) -> float` | Axis value; an unknown axis or a missing controller gives `0.0` |
| `gamepad_buttons() -> string[]` | Every button held right now, as `"<pad>:<button>"` — the quickest way to learn an unmapped controller's numbers |
| `add_gamepad_mapping(lines: string)` | Teaches the engine a controller it does not know (see below) |

The numbering follows the order controllers are connected, so the first one
is always pad `0`.

**Controllers with a known layout** (`gamepad_standard(pad)` is `true`) use
names: `"a"` `"b"` `"x"` `"y"` (the right cluster, bottom/right/left/top),
`"up"` `"down"` `"left"` `"right"` (d-pad), `"start"` `"back"` `"guide"`,
`"lb"` `"rb"` (shoulders), `"lt"` `"rt"` (triggers, also readable as axes),
`"lstick"` `"rstick"` (stick clicks). Axes: `"left_x"` `"left_y"`
`"right_x"` `"right_y"` (−1..1) and `"lt"` `"rt"` (0..1).

**Controllers without one** — many generic USB pads — still work, with the
raw names the device reports: buttons `"b0"`, `"b1"`, ... and axes `"a0"`,
`"a1"`, ... Which number is which button varies by device, so print
`gamepad_buttons()` while pressing them (that is what `showcase.nx` shows on
its bottom line) and use the numbers you see.

To get the nice names on such a controller, pass it a mapping line in
[SDL_GameControllerDB](https://github.com/gabomdq/SDL_GameControllerDB)
format — look yours up by the GUID, or write one — before the loop:

```noxy
game.add_gamepad_mapping("03000000790000000600000000000000,My Pad,platform:Windows,a:b2,b:b1,x:b3,y:b0,start:b9,back:b8,leftx:a0,lefty:a1,")
```

From then on `gamepad_standard(0)` is `true` and the standard names work. It
raises if the text does not parse.

### Camera and collision

These are the one part of the API that never talks to the process: the
camera is applied as commands are queued, and the collision helpers are
plain arithmetic in the wrapper.

| Function | Description |
|---|---|
| `camera_set(x, y)` | Shifts every draw call by `(-x, -y)` — pass the world position you want at the top-left corner |
| `camera_reset()` | Back to no offset |
| `camera_pos() -> Point` | The current offset |
| `rect_overlaps(ax, ay, aw, ah, bx, by, bw, bh) -> bool` | Do the two rectangles overlap? |
| `circle_overlaps(ax, ay, ar, bx, by, br) -> bool` | Do the two circles overlap? |
| `circle_rect_overlaps(cx, cy, r, x, y, w, h) -> bool` | Does the circle touch the rectangle? |
| `point_in_rect(px, py, x, y, w, h) -> bool` | Is the point inside the rectangle? |

The usual shape of a frame: draw the world, call `camera_reset()`, draw the
UI. `clear` is never shifted, and `mouse_pos()` stays in screen coordinates —
add the camera offset to get the position in the world (`showcase.nx` does
exactly that).

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

Audio, `text_width`, `load_font` and the window calls are calls of their
own, outside that batch — they are occasional, not per-frame. `play` returns
immediately (the mixer runs on its own), so a sound never costs you a frame.
The camera and the collision helpers cost nothing at all: they never leave
the wrapper.

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
