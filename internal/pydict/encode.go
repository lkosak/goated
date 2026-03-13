package pydict

import (
	"fmt"
	"sort"
	"strings"
)

// Encode converts a Go map to a Python dict literal string.
// Multiline strings use triple-double-quotes. Keys are sorted for determinism.
func Encode(v map[string]any) string {
	return encodeDict(v, 0)
}

// KV is an ordered key-value pair for use with EncodeOrdered.
type KV struct {
	Key   string
	Value any
}

// EncodeOrdered converts an ordered slice of key-value pairs to a Python dict
// literal string, preserving the given key order.
func EncodeOrdered(pairs []KV) string {
	return encodeDictOrdered(pairs, 0)
}

func encodeDictOrdered(pairs []KV, indent int) string {
	if len(pairs) == 0 {
		return "{}"
	}

	pad := strings.Repeat("  ", indent+1)
	closePad := strings.Repeat("  ", indent)

	var buf strings.Builder
	buf.WriteString("{\n")
	for _, kv := range pairs {
		buf.WriteString(pad)
		buf.WriteString(encodeString(kv.Key))
		buf.WriteString(": ")
		buf.WriteString(encodeValue(kv.Value, indent+1))
		buf.WriteString(",\n")
	}
	buf.WriteString(closePad)
	buf.WriteByte('}')
	return buf.String()
}

func encodeValue(v any, indent int) string {
	switch val := v.(type) {
	case nil:
		return "None"
	case bool:
		if val {
			return "True"
		}
		return "False"
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		// Use %g for compact representation, but avoid scientific notation for common values
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case string:
		return encodeString(val)
	case map[string]any:
		return encodeDict(val, indent)
	case []any:
		return encodeList(val, indent)
	default:
		return fmt.Sprintf("%q", fmt.Sprint(val))
	}
}

func encodeString(s string) string {
	if strings.Contains(s, "\n") {
		// Use triple quotes for multiline strings
		// Choose quote style to avoid conflicts
		if !strings.Contains(s, `"""`) {
			return `"""` + s + `"""`
		}
		if !strings.Contains(s, `'''`) {
			return `'''` + s + `'''`
		}
		// Both present (extremely rare) — escape the double triple-quotes
		escaped := strings.ReplaceAll(s, `"""`, `\"\"\"`)
		return `"""` + escaped + `"""`
	}

	// Single-line string — use double quotes with escaping
	var buf strings.Builder
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\t':
			buf.WriteString(`\t`)
		case '\r':
			buf.WriteString(`\r`)
		default:
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
	return buf.String()
}

func encodeDict(m map[string]any, indent int) string {
	if len(m) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pad := strings.Repeat("  ", indent+1)
	closePad := strings.Repeat("  ", indent)

	var buf strings.Builder
	buf.WriteString("{\n")
	for _, k := range keys {
		v := m[k]
		buf.WriteString(pad)
		buf.WriteString(encodeString(k))
		buf.WriteString(": ")
		buf.WriteString(encodeValue(v, indent+1))
		buf.WriteString(",\n")
	}
	buf.WriteString(closePad)
	buf.WriteByte('}')
	return buf.String()
}

func encodeList(l []any, indent int) string {
	if len(l) == 0 {
		return "[]"
	}

	pad := strings.Repeat("  ", indent+1)
	closePad := strings.Repeat("  ", indent)

	var buf strings.Builder
	buf.WriteString("[\n")
	for _, v := range l {
		buf.WriteString(pad)
		buf.WriteString(encodeValue(v, indent+1))
		buf.WriteString(",\n")
	}
	buf.WriteString(closePad)
	buf.WriteByte(']')
	return buf.String()
}
