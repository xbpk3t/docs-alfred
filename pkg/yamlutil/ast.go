// Package yamlutil provides small helpers around goccy/go-yaml AST nodes.
package yamlutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
)

// NodeLine returns a node's 1-based source line when available.
func NodeLine(n ast.Node) int {
	if n == nil {
		return 0
	}
	tk := n.GetToken()
	if tk == nil {
		return 0
	}

	return tk.Position.Line
}

// KeyString returns the comparable string representation of a mapping key.
func KeyString(n ast.MapKeyNode) string {
	if n == nil {
		return ""
	}
	switch v := n.(type) {
	case *ast.StringNode:
		return v.Value
	case *ast.IntegerNode:
		switch val := v.Value.(type) {
		case int64:
			return strconv.FormatInt(val, 10)
		case uint64:
			return strconv.FormatUint(val, 10)
		}

		return fmt.Sprintf("%v", v.Value)
	default:
		return n.String()
	}
}

// MappingValue returns the value for key in m.
func MappingValue(m *ast.MappingNode, key string) ast.Node {
	if m == nil {
		return nil
	}
	for _, v := range m.Values {
		if v == nil {
			continue
		}
		if KeyString(v.Key) == key {
			return v.Value
		}
	}

	return nil
}

// Mapping returns n as a MappingNode.
func Mapping(n ast.Node) (*ast.MappingNode, bool) {
	m, ok := n.(*ast.MappingNode)

	return m, ok
}

// Sequence returns n as a SequenceNode.
func Sequence(n ast.Node) (*ast.SequenceNode, bool) {
	seq, ok := n.(*ast.SequenceNode)

	return seq, ok
}

// String returns n as a StringNode value.
func String(n ast.Node) (string, bool) {
	s, ok := n.(*ast.StringNode)
	if !ok {
		return "", false
	}

	return s.Value, true
}

// IsNullOrEmptyString reports whether n is null, nil, or an empty string node.
func IsNullOrEmptyString(n ast.Node) bool {
	if n == nil {
		return true
	}
	if _, ok := n.(*ast.NullNode); ok {
		return true
	}
	if s, ok := n.(*ast.StringNode); ok && strings.TrimSpace(s.Value) == "" {
		return true
	}

	return false
}
