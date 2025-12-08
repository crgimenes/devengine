package filo

import (
	"fmt"
	"strconv"
	"strings"
)

type Node interface{}

type NumberLit struct {
	Value float64
}

type BoolLit struct {
	Value bool
}

type StringLit struct {
	Value string
}

type Symbol struct {
	Name string
}

type List struct {
	Elems []Node
}

type lexer struct {
	src string
	i   int
}

func parse(src string) (Node, error) {
	lx := &lexer{src: src}
	var nodes []Node

	// Read all top-level expressions
	for {
		lx.skipWS()
		if lx.i >= len(lx.src) {
			break
		}
		node, err := lx.readNode()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	// If no nodes, return error
	if len(nodes) == 0 {
		return nil, fmt.Errorf("empty script")
	}

	// If single node, return it directly
	if len(nodes) == 1 {
		return nodes[0], nil
	}

	// Multiple nodes: wrap in implicit (let () ...) block
	// This allows sequential evaluation with the last value returned
	return &List{Elems: append([]Node{&Symbol{Name: "let"}, &List{Elems: []Node{}}}, nodes...)}, nil
}

func (l *lexer) readNode() (Node, error) {
	l.skipWS()
	if l.i >= len(l.src) {
		return nil, fmt.Errorf("unexpected end of input")
	}

	c := l.src[l.i]
	switch c {
	case '(':
		l.i++
		return l.readList()
	case '"':
		return l.readString()
	case '#':
		return l.readBool()
	default:
		return l.readAtom()
	}
}

func (l *lexer) skipWS() {
	for l.i < len(l.src) {
		switch l.src[l.i] {
		case ' ', '\t', '\n', '\r':
			l.i++
		case ';':
			for l.i < len(l.src) && l.src[l.i] != '\n' {
				l.i++
			}
		default:
			return
		}
	}
}

func (l *lexer) readList() (Node, error) {
	var elems []Node
	for {
		l.skipWS()
		if l.i >= len(l.src) {
			return nil, fmt.Errorf("unterminated list")
		}
		if l.src[l.i] == ')' {
			l.i++
			break
		}
		node, err := l.readNode()
		if err != nil {
			return nil, err
		}
		elems = append(elems, node)
	}
	return &List{Elems: elems}, nil
}

func (l *lexer) readString() (Node, error) {
	l.i++
	var b strings.Builder
	for l.i < len(l.src) {
		c := l.src[l.i]
		if c == '"' {
			l.i++
			return &StringLit{Value: b.String()}, nil
		}
		if c == '\\' {
			l.i++
			if l.i >= len(l.src) {
				return nil, fmt.Errorf("unterminated escape sequence")
			}
			escaped := l.src[l.i]
			switch escaped {
			case '"':
				b.WriteByte('"')
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			default:
				return nil, fmt.Errorf("unsupported escape: \\%c", escaped)
			}
			l.i++
			continue
		}
		b.WriteByte(c)
		l.i++
	}
	return nil, fmt.Errorf("unterminated string literal")
}

func (l *lexer) readBool() (Node, error) {
	if strings.HasPrefix(l.src[l.i:], "#t") {
		l.i += 2
		return &BoolLit{Value: true}, nil
	}
	if strings.HasPrefix(l.src[l.i:], "#f") {
		l.i += 2
		return &BoolLit{Value: false}, nil
	}
	return nil, fmt.Errorf("invalid boolean literal at position %d", l.i)
}

func (l *lexer) readAtom() (Node, error) {
	start := l.i
	for l.i < len(l.src) {
		switch l.src[l.i] {
		case ' ', '\t', '\n', '\r', '(', ')':
			goto done
		default:
			l.i++
		}
	}
done:
	text := l.src[start:l.i]
	num, err := strconv.ParseFloat(text, 64)
	if err == nil {
		return &NumberLit{Value: num}, nil
	}
	return &Symbol{Name: text}, nil
}
