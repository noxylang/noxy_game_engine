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
