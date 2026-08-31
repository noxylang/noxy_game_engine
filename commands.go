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
	cmdPolygon
)

type color4 struct{ R, G, B, A uint8 }

// command é um desenho decodificado. Campos por tag:
//
//	clear:  color
//	rect:   x y w h color thickness (0 = preenchido)
//	circle: x y (centro) w (raio) color thickness
//	line:   x y (início) w h (fim) color thickness
//	text:   text x y size color font (0 = embutida)
//	image:  image srcX srcY srcW srcH x y scale angle (graus) opacity flipX
//	polygon: points color thickness
type command struct {
	kind                                      cmdKind
	color                                     color4
	x, y, w, h, thickness, size, scale, angle float64
	srcX, srcY, srcW, srcH, opacity           float64
	flipX                                     bool
	points                                    []float64
	text                                      string
	image, font                               int64
}

// arity é o tamanho exato (tag incluída) de cada comando.
var arity = map[string]int{
	"clear": 5, "rect": 10, "circle": 9, "line": 10, "text": 10, "image": 12, "polygon": 7,
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
		c.thickness = r.thickness(9)
	case "circle":
		c.kind = cmdCircle
		c.x, c.y, c.w = r.num(1), r.num(2), r.num(3)
		c.color = r.color(4)
		c.thickness = r.thickness(8)
	case "line":
		c.kind = cmdLine
		c.x, c.y, c.w, c.h = r.num(1), r.num(2), r.num(3), r.num(4)
		c.color = r.color(5)
		c.thickness = r.thickness(9)
	case "text":
		c.kind = cmdText
		c.text = r.str(1)
		c.x, c.y, c.size = r.num(2), r.num(3), r.num(4)
		c.color = r.color(5)
		c.font = r.integer(9)
	case "image":
		c.kind = cmdImage
		c.image = r.integer(1)
		c.srcX, c.srcY = r.num(2), r.num(3)
		c.srcW, c.srcH = r.positive(4), r.positive(5)
		c.x, c.y = r.num(6), r.num(7)
		c.scale, c.angle = r.num(8), r.num(9)
		c.opacity = r.opacity(10)
		c.flipX = r.boolean(11)
	case "polygon":
		c.kind = cmdPolygon
		c.points = r.points(1)
		c.color = r.color(2)
		c.thickness = r.thickness(6)
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

// positive lê um tamanho de sub-retângulo (largura/altura de origem).
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

func (r *reader) thickness(i int) float64 {
	t := r.num(i)
	if t < 0 {
		r.fail(i, "thickness must not be negative, got %g", t)
	}
	return t
}

// color lê r, g, b, a nos elementos i..i+3.
func (r *reader) color(i int) color4 {
	var out [4]uint8
	for k := 0; k < 4; k++ {
		idx := i + k
		var n int64
		switch v := r.parts[idx].(type) {
		case int64:
			n = v
		case int:
			n = int64(v)
		default:
			r.fail(idx, "color component must be an int, got %s", typeName(v))
			return color4{}
		}
		if n < 0 || n > 255 {
			r.fail(idx, "color component out of range 0..255, got %d", n)
			return color4{}
		}
		out[k] = uint8(n)
	}
	return color4{out[0], out[1], out[2], out[3]}
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
