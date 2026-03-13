// Package pydict parses and encodes Python dict literal syntax.
// It handles a superset of JSON: triple-quoted strings, single quotes,
// trailing commas, True/False/None.
package pydict

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Parse parses a Python dict literal string into a Go map.
func Parse(input string) (map[string]any, error) {
	p := &parser{input: []rune(input)}
	p.skipWhitespace()
	v, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("pydict: top-level value must be a dict, got %T", v)
	}
	return m, nil
}

type parser struct {
	input []rune
	pos   int
}

func (p *parser) peek() (rune, bool) {
	if p.pos >= len(p.input) {
		return 0, false
	}
	return p.input[p.pos], true
}

func (p *parser) advance() rune {
	r := p.input[p.pos]
	p.pos++
	return r
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) {
		r := p.input[p.pos]
		if r == '#' {
			// skip comment to end of line
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		if !unicode.IsSpace(r) {
			break
		}
		p.pos++
	}
}

func (p *parser) parseValue() (any, error) {
	p.skipWhitespace()
	ch, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("pydict: unexpected end of input")
	}

	switch {
	case ch == '{':
		return p.parseDict()
	case ch == '[':
		return p.parseList()
	case ch == '"' || ch == '\'':
		return p.parseString()
	case ch == '-' || (ch >= '0' && ch <= '9'):
		return p.parseNumber()
	case ch == 'T' || ch == 'F' || ch == 'N' || ch == 't' || ch == 'f' || ch == 'n':
		return p.parseKeyword()
	default:
		return nil, fmt.Errorf("pydict: unexpected character %q at position %d", ch, p.pos)
	}
}

func (p *parser) parseDict() (map[string]any, error) {
	p.advance() // consume '{'
	result := make(map[string]any)

	for {
		p.skipWhitespace()
		ch, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("pydict: unterminated dict")
		}
		if ch == '}' {
			p.advance()
			return result, nil
		}

		// Parse key
		key, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("pydict: dict key: %w", err)
		}
		keyStr, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("pydict: dict key must be string, got %T", key)
		}

		// Expect ':'
		p.skipWhitespace()
		ch, ok = p.peek()
		if !ok || ch != ':' {
			return nil, fmt.Errorf("pydict: expected ':' after dict key %q at position %d", keyStr, p.pos)
		}
		p.advance()

		// Parse value
		val, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("pydict: dict value for key %q: %w", keyStr, err)
		}
		result[keyStr] = val

		// Expect ',' or '}'
		p.skipWhitespace()
		ch, ok = p.peek()
		if !ok {
			return nil, fmt.Errorf("pydict: unterminated dict")
		}
		if ch == ',' {
			p.advance()
			continue
		}
		if ch == '}' {
			continue // will be consumed at top of loop
		}
		return nil, fmt.Errorf("pydict: expected ',' or '}' in dict at position %d", p.pos)
	}
}

func (p *parser) parseList() ([]any, error) {
	p.advance() // consume '['
	var result []any

	for {
		p.skipWhitespace()
		ch, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("pydict: unterminated list")
		}
		if ch == ']' {
			p.advance()
			return result, nil
		}

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		result = append(result, val)

		p.skipWhitespace()
		ch, ok = p.peek()
		if !ok {
			return nil, fmt.Errorf("pydict: unterminated list")
		}
		if ch == ',' {
			p.advance()
			continue
		}
		if ch == ']' {
			continue
		}
		return nil, fmt.Errorf("pydict: expected ',' or ']' in list at position %d", p.pos)
	}
}

func (p *parser) parseString() (string, error) {
	quote := p.advance() // consume opening quote

	// Check for triple quote
	if p.pos+1 < len(p.input) && p.input[p.pos] == quote && p.input[p.pos+1] == quote {
		p.pos += 2 // consume the other two quotes
		return p.parseTripleString(quote)
	}

	return p.parseSingleString(quote)
}

func (p *parser) parseSingleString(quote rune) (string, error) {
	var buf strings.Builder
	for p.pos < len(p.input) {
		ch := p.advance()
		if ch == quote {
			return buf.String(), nil
		}
		if ch == '\\' {
			esc, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			buf.WriteRune(esc)
			continue
		}
		buf.WriteRune(ch)
	}
	return "", fmt.Errorf("pydict: unterminated string")
}

func (p *parser) parseTripleString(quote rune) (string, error) {
	var buf strings.Builder
	closer := string([]rune{quote, quote, quote})

	for p.pos < len(p.input) {
		// Check for closing triple quote
		if p.pos+2 < len(p.input) &&
			p.input[p.pos] == quote &&
			p.input[p.pos+1] == quote &&
			p.input[p.pos+2] == quote {
			p.pos += 3
			return buf.String(), nil
		}
		ch := p.advance()
		if ch == '\\' {
			esc, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			buf.WriteRune(esc)
			continue
		}
		buf.WriteRune(ch)
	}
	return "", fmt.Errorf("pydict: unterminated triple-quoted string (looking for %s)", closer)
}

func (p *parser) parseEscape() (rune, error) {
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("pydict: unexpected end after backslash")
	}
	ch := p.advance()
	switch ch {
	case 'n':
		return '\n', nil
	case 't':
		return '\t', nil
	case 'r':
		return '\r', nil
	case '\\':
		return '\\', nil
	case '\'':
		return '\'', nil
	case '"':
		return '"', nil
	case '/':
		return '/', nil
	default:
		// Pass through unknown escapes as-is
		return ch, nil
	}
}

func (p *parser) parseNumber() (any, error) {
	start := p.pos
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
	}
	isFloat := false
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		isFloat = true
		p.pos++
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		isFloat = true
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}

	s := string(p.input[start:p.pos])
	if isFloat {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("pydict: invalid float %q: %w", s, err)
		}
		return f, nil
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("pydict: invalid int %q: %w", s, err)
	}
	return i, nil
}

func (p *parser) parseKeyword() (any, error) {
	// Read identifier
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(p.input[p.pos]) || p.input[p.pos] == '_') {
		p.pos++
	}
	word := string(p.input[start:p.pos])
	switch word {
	case "True", "true":
		return true, nil
	case "False", "false":
		return false, nil
	case "None", "null", "nil":
		return nil, nil
	default:
		return nil, fmt.Errorf("pydict: unknown keyword %q at position %d", word, start)
	}
}
