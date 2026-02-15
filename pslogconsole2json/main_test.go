package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTypeTracker(t *testing.T) {
	t.Run("quoted value pins key to string", func(t *testing.T) {
		tracker := newTypeTracker()
		if got := tracker.parseValue("id", "001", true); got != "001" {
			t.Fatalf("expected quoted value to stay string, got %T %v", got, got)
		}
		if got := tracker.parseValue("id", "2", false); got != "2" {
			t.Fatalf("expected key pinned to string, got %T %v", got, got)
		}
	})

	t.Run("invalid numeric fallback pins to string", func(t *testing.T) {
		tracker := newTypeTracker()
		if got := tracker.parseValue("n", "10", false); got != int64(10) {
			t.Fatalf("expected int64(10), got %T %v", got, got)
		}
		if got := tracker.parseValue("n", "oops", false); got != "oops" {
			t.Fatalf("expected fallback string, got %T %v", got, got)
		}
		if got := tracker.parseValue("n", "12", false); got != "12" {
			t.Fatalf("expected pinned string after fallback, got %T %v", got, got)
		}
	})
}

func TestParseDTG(t *testing.T) {
	loc := time.UTC

	now := time.Date(2026, time.February, 10, 9, 0, 0, 0, loc)
	got, ok := parseDTG("101530", now)
	if !ok {
		t.Fatalf("expected valid DTG")
	}
	if got != "2026-02-10T15:30:00Z" {
		t.Fatalf("unexpected DTG parse, got %q", got)
	}

	nowApr := time.Date(2026, time.April, 10, 9, 0, 0, 0, loc)
	if _, ok := parseDTG("311530", nowApr); ok {
		t.Fatalf("expected April 31 to be rejected")
	}

	leapNow := time.Date(2024, time.February, 10, 9, 0, 0, 0, loc)
	if _, ok := parseDTG("291200", leapNow); !ok {
		t.Fatalf("expected Feb 29 in leap year to be accepted")
	}

	nonLeapNow := time.Date(2023, time.February, 10, 9, 0, 0, 0, loc)
	if _, ok := parseDTG("291200", nonLeapNow); ok {
		t.Fatalf("expected Feb 29 in non-leap year to be rejected")
	}
}

func TestParseFields(t *testing.T) {
	tracker := newTypeTracker()
	fields, err := parseFields(`a=1 b=true c=3.14 d=nil e="hello world" f=`, tracker)
	if err != nil {
		t.Fatalf("parseFields returned unexpected error: %v", err)
	}
	if len(fields) != 6 {
		t.Fatalf("expected 6 fields, got %d", len(fields))
	}
	if fields[0].key != "a" || fields[0].value != int64(1) {
		t.Fatalf("unexpected a field: %+v", fields[0])
	}
	if fields[1].key != "b" || fields[1].value != true {
		t.Fatalf("unexpected b field: %+v", fields[1])
	}
	if fields[4].key != "e" || fields[4].value != "hello world" {
		t.Fatalf("unexpected e field: %+v", fields[4])
	}
	if fields[5].key != "f" || fields[5].value != "" {
		t.Fatalf("unexpected f field: %+v", fields[5])
	}

	if _, err := parseFields(`a=1 broken tail=2`, newTypeTracker()); err == nil {
		t.Fatalf("expected missing '=' tail error")
	}
	if _, err := parseFields(`=1`, newTypeTracker()); err == nil {
		t.Fatalf("expected empty key error")
	}
	if _, err := parseFields(`a="unterminated`, newTypeTracker()); err == nil {
		t.Fatalf("expected unterminated quote error")
	}
}

func TestParseConsoleLine(t *testing.T) {
	now := time.Date(2026, time.February, 10, 10, 0, 0, 0, time.UTC)
	doc, fields, err := parseConsoleLine(`101530 INF hello a=1 ok=true user="alice"`, newTypeTracker(), now)
	if err != nil {
		t.Fatalf("parseConsoleLine returned unexpected error: %v", err)
	}
	if doc["ts"] != "2026-02-10T15:30:00Z" {
		t.Fatalf("unexpected ts: %v", doc["ts"])
	}
	if doc["lvl"] != "info" {
		t.Fatalf("unexpected lvl: %v", doc["lvl"])
	}
	if doc["msg"] != "hello" {
		t.Fatalf("unexpected msg: %v", doc["msg"])
	}
	if doc["a"] != int64(1) {
		t.Fatalf("unexpected a type/value: %T %v", doc["a"], doc["a"])
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 parsed fields, got %d", len(fields))
	}

	if _, _, err := parseConsoleLine(`INF bad a=1 broken`, newTypeTracker(), now); err == nil {
		t.Fatalf("expected malformed fields to return error")
	}
}

func TestProcessReader(t *testing.T) {
	now := time.Date(2026, time.February, 10, 10, 0, 0, 0, time.UTC)
	filter := selectorFilter{enabled: false}

	var firstOut bytes.Buffer
	if err := processReader(strings.NewReader("INF one id=\"001\"\n"), "first", &firstOut, filter, now); err != nil {
		t.Fatalf("processReader first run failed: %v", err)
	}
	if !strings.Contains(firstOut.String(), `"id":"001"`) {
		t.Fatalf("expected first run id to be string, got %q", firstOut.String())
	}

	var secondOut bytes.Buffer
	if err := processReader(strings.NewReader("INF two id=2\n"), "second", &secondOut, filter, now); err != nil {
		t.Fatalf("processReader second run failed: %v", err)
	}
	if !strings.Contains(secondOut.String(), `"id":2`) {
		t.Fatalf("expected second run id to be numeric (tracker isolation), got %q", secondOut.String())
	}
}

func TestFilter(t *testing.T) {
	now := time.Date(2026, time.February, 10, 10, 0, 0, 0, time.UTC)
	filter, err := newSelectorFilter([]string{"/lvl=error"}, false)
	if err != nil {
		t.Fatalf("newSelectorFilter failed: %v", err)
	}

	var out bytes.Buffer
	input := "INF ok code=200\nERR bad code=500\n"
	if err := processReader(strings.NewReader(input), "filter", &out, filter, now); err != nil {
		t.Fatalf("processReader failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one filtered line, got %d (%q)", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"lvl":"error"`) {
		t.Fatalf("unexpected filtered output: %q", lines[0])
	}
}

func TestOutputOrder(t *testing.T) {
	doc := map[string]any{
		"msg": "hello",
		"lvl": "info",
		"ts":  "2026-02-10T12:00:00Z",
		"a":   int64(1),
		"b":   true,
	}
	fields := []fieldPair{
		{key: "a", value: int64(1)},
		{key: "b", value: true},
	}

	var out bytes.Buffer
	if err := writeOrderedJSON(&out, doc, fields); err != nil {
		t.Fatalf("writeOrderedJSON failed: %v", err)
	}

	got := out.String()
	want := `{"ts":"2026-02-10T12:00:00Z","lvl":"info","msg":"hello","a":1,"b":true}` + "\n"
	if got != want {
		t.Fatalf("unexpected JSON output order:\nwant: %q\ngot:  %q", want, got)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid json: %v", err)
	}
}

func TestGeneratedConsoleCorpusProcessReader(t *testing.T) {
	now := time.Date(2026, time.February, 10, 10, 0, 0, 0, time.UTC)
	path := filepath.Join("testdata", "generated_console_matrix.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	inputLines := nonEmptyLines(string(data))
	if len(inputLines) == 0 {
		t.Fatalf("fixture has no input lines")
	}

	var out bytes.Buffer
	if err := processReader(bytes.NewReader(data), path, &out, selectorFilter{enabled: false}, now); err != nil {
		t.Fatalf("processReader failed: %v", err)
	}

	outputLines := nonEmptyLines(out.String())
	if len(outputLines) != len(inputLines) {
		t.Fatalf("expected all fixture lines to convert, input=%d output=%d", len(inputLines), len(outputLines))
	}

	seenTS := 0
	seenArg26 := 0
	for i, line := range outputLines {
		doc := decodeJSONDocUseNumber(t, line)
		if _, ok := doc["lvl"]; !ok {
			t.Fatalf("line %d missing lvl: %s", i+1, line)
		}
		if _, ok := doc["msg"]; !ok {
			t.Fatalf("line %d missing msg: %s", i+1, line)
		}
		if _, ok := doc["ts"]; ok {
			seenTS++
		}
		if _, ok := doc["arg26"]; ok {
			seenArg26++
		}
		assertTypedFields(t, doc, i)
	}
	if seenTS == 0 {
		t.Fatalf("expected timestamp-bearing lines in converted output")
	}
	if seenArg26 == 0 {
		t.Fatalf("expected odd-tail arg26 fields in converted output")
	}
}

func TestGeneratedConsoleCorpusFilter(t *testing.T) {
	now := time.Date(2026, time.February, 10, 10, 0, 0, 0, time.UTC)
	path := filepath.Join("testdata", "generated_console_matrix.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	filter, err := newSelectorFilter([]string{"/lvl=error"}, false)
	if err != nil {
		t.Fatalf("selector parse failed: %v", err)
	}

	var out bytes.Buffer
	if err := processReader(bytes.NewReader(data), path, &out, filter, now); err != nil {
		t.Fatalf("processReader failed: %v", err)
	}

	lines := nonEmptyLines(out.String())
	if len(lines) == 0 {
		t.Fatalf("expected at least one filtered line")
	}
	for i, line := range lines {
		doc := decodeJSONDocUseNumber(t, line)
		lvl, _ := doc["lvl"].(string)
		if lvl != "error" {
			t.Fatalf("line %d expected lvl=error, got %q (%s)", i+1, lvl, line)
		}
	}
}

func nonEmptyLines(s string) []string {
	raw := strings.Split(strings.TrimSpace(s), "\n")
	if len(raw) == 1 && raw[0] == "" {
		return nil
	}
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func decodeJSONDocUseNumber(t *testing.T, line string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode json line %q: %v", line, err)
	}
	return doc
}

func assertTypedFields(t *testing.T, doc map[string]any, lineIdx int) {
	t.Helper()

	if v, ok := doc["str"].(string); !ok || v != "value" {
		t.Fatalf("line %d unexpected str: %#v", lineIdx+1, doc["str"])
	}
	if v, ok := doc["sp"].(string); !ok || v != "hello world" {
		t.Fatalf("line %d unexpected sp: %#v", lineIdx+1, doc["sp"])
	}
	if v, ok := doc["trusted"].(string); !ok || v != "trusted_value" {
		t.Fatalf("line %d unexpected trusted: %#v", lineIdx+1, doc["trusted"])
	}
	if v, ok := doc["quote"].(string); !ok || v != `say "hi"` {
		t.Fatalf("line %d unexpected quote: %#v", lineIdx+1, doc["quote"])
	}
	if v, ok := doc["bool_t"].(bool); !ok || !v {
		t.Fatalf("line %d unexpected bool_t: %#v", lineIdx+1, doc["bool_t"])
	}
	if v, ok := doc["bool_f"].(bool); !ok || v {
		t.Fatalf("line %d unexpected bool_f: %#v", lineIdx+1, doc["bool_f"])
	}
	if _, ok := doc["nilv"]; !ok || doc["nilv"] != nil {
		t.Fatalf("line %d unexpected nilv: %#v", lineIdx+1, doc["nilv"])
	}
	if v, ok := doc["bytes"].(string); !ok || v != "blob" {
		t.Fatalf("line %d unexpected bytes: %#v", lineIdx+1, doc["bytes"])
	}
	if v, ok := doc["dur"].(string); !ok || v == "" {
		t.Fatalf("line %d unexpected dur: %#v", lineIdx+1, doc["dur"])
	}
	if v, ok := doc["time"].(string); !ok || !strings.HasPrefix(v, "2026-02-10T12:") {
		t.Fatalf("line %d unexpected time: %#v", lineIdx+1, doc["time"])
	}
	if v, ok := doc["err"].(string); !ok || v != "boom" {
		t.Fatalf("line %d unexpected err: %#v", lineIdx+1, doc["err"])
	}
	if v, ok := doc["stringer"].(string); !ok || !strings.HasPrefix(v, "stringer_") {
		t.Fatalf("line %d unexpected stringer: %#v", lineIdx+1, doc["stringer"])
	}
	if v, ok := doc["json"].(string); !ok || !strings.Contains(v, `"k":"v"`) {
		t.Fatalf("line %d unexpected json field: %#v", lineIdx+1, doc["json"])
	}
	if v, ok := doc["123"].(string); !ok || v != "numkey" {
		t.Fatalf("line %d unexpected numeric-key field: %#v", lineIdx+1, doc["123"])
	}

	assertJSONNumberField(t, doc, lineIdx, "int")
	assertJSONNumberField(t, doc, lineIdx, "int8")
	assertJSONNumberField(t, doc, lineIdx, "int16")
	assertJSONNumberField(t, doc, lineIdx, "int32")
	assertJSONNumberField(t, doc, lineIdx, "int64")
	assertJSONNumberField(t, doc, lineIdx, "uint")
	assertJSONNumberField(t, doc, lineIdx, "uint8")
	assertJSONNumberField(t, doc, lineIdx, "uint16")
	assertJSONNumberField(t, doc, lineIdx, "uint32")
	assertJSONNumberField(t, doc, lineIdx, "uint64")
	assertJSONNumberField(t, doc, lineIdx, "float32")
	assertJSONNumberField(t, doc, lineIdx, "float64")
}

func assertJSONNumberField(t *testing.T, doc map[string]any, lineIdx int, key string) {
	t.Helper()
	if _, ok := doc[key].(json.Number); !ok {
		t.Fatalf("line %d expected %q as json.Number, got %T (%#v)", lineIdx+1, key, doc[key], doc[key])
	}
}
