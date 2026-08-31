package main

import (
	"strings"
	"testing"
)

func TestDecodeFrameAllTags(t *testing.T) {
	raw := []any{
		[]any{"clear", int64(1), int64(2), int64(3), int64(255)},
		[]any{"rect", 1.5, int64(2), 3.0, 4.0, int64(255), int64(0), int64(0), int64(255), int64(0)},
		[]any{"circle", 10.0, 20.0, 5.0, int64(0), int64(255), int64(0), int64(255), 2.0},
		[]any{"line", 0.0, 0.0, 9.0, 9.0, int64(0), int64(0), int64(255), int64(255), 1.0},
		[]any{"text", "hi", 3.0, 4.0, int64(16), int64(9), int64(9), int64(9), int64(255), int64(0)},
		[]any{"image", int64(7), 0.0, 0.0, 32.0, 32.0, 1.0, 2.0, 2.0, 90.0, 1.0, false},
	}
	cmds, err := decodeFrame(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 6 {
		t.Fatalf("got %d commands", len(cmds))
	}
	if cmds[0].kind != cmdClear || cmds[0].color != (color4{1, 2, 3, 255}) {
		t.Errorf("clear: %+v", cmds[0])
	}
	r := cmds[1]
	if r.kind != cmdRect || r.x != 1.5 || r.y != 2 || r.w != 3 || r.h != 4 || r.color != (color4{255, 0, 0, 255}) || r.thickness != 0 {
		t.Errorf("rect: %+v", r)
	}
	c := cmds[2]
	if c.kind != cmdCircle || c.x != 10 || c.y != 20 || c.w != 5 || c.thickness != 2 {
		t.Errorf("circle: %+v", c)
	}
	l := cmds[3]
	if l.kind != cmdLine || l.w != 9 || l.h != 9 || l.thickness != 1 || l.color != (color4{0, 0, 255, 255}) {
		t.Errorf("line: %+v", l)
	}
	tx := cmds[4]
	if tx.kind != cmdText || tx.text != "hi" || tx.x != 3 || tx.size != 16 || tx.color != (color4{9, 9, 9, 255}) {
		t.Errorf("text: %+v", tx)
	}
	im := cmds[5]
	if im.kind != cmdImage || im.image != 7 || im.x != 1 || im.y != 2 || im.scale != 2 || im.angle != 90 ||
		im.srcW != 32 || im.srcH != 32 || im.opacity != 1 || im.flipX {
		t.Errorf("image: %+v", im)
	}
}

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
		{"arity", []any{[]any{"clear", int64(1)}}, `command 0: "clear" expects 5 elements, got 2`},
		{"number type", []any{[]any{"clear", int64(1), int64(2), int64(3), int64(255)}, []any{"rect", "x", 1.0, 1.0, 1.0, int64(0), int64(0), int64(0), int64(255), int64(0)}}, `command 1: element 1 of "rect": expected number, got string`},
		{"color range", []any{[]any{"clear", int64(300), int64(0), int64(0), int64(255)}}, `command 0: element 1 of "clear": color component out of range 0..255, got 300`},
		{"color float", []any{[]any{"clear", 1.5, int64(0), int64(0), int64(255)}}, `command 0: element 1 of "clear": color component must be an int, got float`},
		{"text type", []any{[]any{"text", int64(1), 0.0, 0.0, int64(12), int64(0), int64(0), int64(0), int64(255), int64(0)}}, `command 0: element 1 of "text": expected string, got int`},
		{"image id type", []any{[]any{"image", "a", 0.0, 0.0, 8.0, 8.0, 0.0, 0.0, 1.0, 0.0, 1.0, false}}, `command 0: element 1 of "image": expected int, got string`},
		{"negative thickness", []any{[]any{"line", 0.0, 0.0, 1.0, 1.0, int64(0), int64(0), int64(0), int64(255), -1.0}}, `command 0: element 9 of "line": thickness must not be negative, got -1`},
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

func TestDecodeAlphaOutOfRange(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"clear", int64(0), int64(0), int64(0), int64(256)}})
	if err == nil || !strings.Contains(err.Error(), "color component out of range 0..255, got 256") {
		t.Fatalf("want alpha range error, got %v", err)
	}
}

func TestDecodeAlphaAccepted(t *testing.T) {
	cmds, err := decodeFrame([]any{[]any{"rect", 1, 2, 3, 4, int64(10), int64(20), int64(30), int64(128), 0}})
	if err != nil {
		t.Fatal(err)
	}
	if cmds[0].color != (color4{10, 20, 30, 128}) {
		t.Fatalf("got %+v", cmds[0].color)
	}
}

func TestDecodeOldColorArityRejected(t *testing.T) {
	_, err := decodeFrame([]any{[]any{"clear", int64(0), int64(0), int64(0)}})
	if err == nil || !strings.Contains(err.Error(), `"clear" expects 5 elements, got 4`) {
		t.Fatalf("want arity error, got %v", err)
	}
}

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
