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
//
//	clear:  color
//	rect:   x y w h color thickness (0 = preenchido)
//	circle: x y (centro) w (raio) color thickness
//	line:   x y (início) w h (fim) color thickness
//	text:   text x y size color
//	image:  image x y scale angle (graus)
type command struct {
	kind                                      cmdKind
	color                                     color3
	x, y, w, h, thickness, size, scale, angle float64
	text                                      string
	image                                     int64
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
