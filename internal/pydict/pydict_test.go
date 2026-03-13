package pydict

import (
	"testing"
)

func TestParseSimpleDict(t *testing.T) {
	input := `{"name": "alice", "age": 30}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
	if m["age"] != int64(30) {
		t.Errorf("age = %v (%T), want 30", m["age"], m["age"])
	}
}

func TestParseSingleQuotes(t *testing.T) {
	input := `{'key': 'value'}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["key"] != "value" {
		t.Errorf("key = %v, want value", m["key"])
	}
}

func TestParseTripleQuotes(t *testing.T) {
	input := `{"text": """hello
world
line 3"""}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	want := "hello\nworld\nline 3"
	if m["text"] != want {
		t.Errorf("text = %q, want %q", m["text"], want)
	}
}

func TestParseTripleSingleQuotes(t *testing.T) {
	input := `{"text": '''contains "double" quotes'''}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	want := `contains "double" quotes`
	if m["text"] != want {
		t.Errorf("text = %q, want %q", m["text"], want)
	}
}

func TestParseTrailingCommas(t *testing.T) {
	input := `{"a": 1, "b": 2,}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["a"] != int64(1) || m["b"] != int64(2) {
		t.Errorf("unexpected values: %v", m)
	}
}

func TestParseBoolsAndNone(t *testing.T) {
	input := `{"a": True, "b": False, "c": None}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["a"] != true {
		t.Errorf("a = %v, want true", m["a"])
	}
	if m["b"] != false {
		t.Errorf("b = %v, want false", m["b"])
	}
	if m["c"] != nil {
		t.Errorf("c = %v, want nil", m["c"])
	}
}

func TestParseNestedDict(t *testing.T) {
	input := `{"outer": {"inner": "value"}}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	outer, ok := m["outer"].(map[string]any)
	if !ok {
		t.Fatalf("outer is %T, want map", m["outer"])
	}
	if outer["inner"] != "value" {
		t.Errorf("inner = %v, want value", outer["inner"])
	}
}

func TestParseList(t *testing.T) {
	input := `{"items": [1, "two", True, None,]}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("items is %T, want []any", m["items"])
	}
	if len(items) != 4 {
		t.Fatalf("len = %d, want 4", len(items))
	}
	if items[0] != int64(1) {
		t.Errorf("[0] = %v", items[0])
	}
	if items[1] != "two" {
		t.Errorf("[1] = %v", items[1])
	}
	if items[2] != true {
		t.Errorf("[2] = %v", items[2])
	}
	if items[3] != nil {
		t.Errorf("[3] = %v", items[3])
	}
}

func TestParseFloat(t *testing.T) {
	input := `{"pi": 3.14, "neg": -1.5}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["pi"] != 3.14 {
		t.Errorf("pi = %v", m["pi"])
	}
	if m["neg"] != -1.5 {
		t.Errorf("neg = %v", m["neg"])
	}
}

func TestRoundTrip(t *testing.T) {
	original := map[string]any{
		"name":    "test",
		"count":   int64(42),
		"enabled": true,
		"message": "hello\nworld\nline 3",
		"tags":    []any{"a", "b"},
	}
	encoded := Encode(original)
	decoded, err := Parse(encoded)
	if err != nil {
		t.Fatalf("parse failed: %v\nencoded:\n%s", err, encoded)
	}
	if decoded["name"] != "test" {
		t.Errorf("name = %v", decoded["name"])
	}
	if decoded["count"] != int64(42) {
		t.Errorf("count = %v (%T)", decoded["count"], decoded["count"])
	}
	if decoded["enabled"] != true {
		t.Errorf("enabled = %v", decoded["enabled"])
	}
	if decoded["message"] != "hello\nworld\nline 3" {
		t.Errorf("message = %q", decoded["message"])
	}
}

func TestEncodeTripleQuoteEscaping(t *testing.T) {
	// String containing """ should use ''' instead
	s := `contains """ inside`
	m := map[string]any{"text": s}
	encoded := Encode(m)
	decoded, err := Parse(encoded)
	if err != nil {
		t.Fatalf("parse failed: %v\nencoded:\n%s", err, encoded)
	}
	if decoded["text"] != s {
		t.Errorf("text = %q, want %q", decoded["text"], s)
	}
}

func TestParseEmptyDict(t *testing.T) {
	m, err := Parse("{}")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty dict, got %v", m)
	}
}

func TestParseComment(t *testing.T) {
	input := `{
		# this is a comment
		"key": "value",  # inline comment
	}`
	m, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["key"] != "value" {
		t.Errorf("key = %v", m["key"])
	}
}
