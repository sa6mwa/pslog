package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pkt.systems/lql"
)

type listFlag []string

func (f *listFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ",")
}

func (f *listFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type valueKind int

const (
	kindUnknown valueKind = iota
	kindString
	kindBool
	kindInt
	kindFloat
	kindNull
)

type typeTracker struct {
	kinds map[string]valueKind
}

func newTypeTracker() *typeTracker {
	return &typeTracker{kinds: make(map[string]valueKind)}
}

func (t *typeTracker) parseValue(key, raw string, quoted bool) any {
	if quoted {
		t.kinds[key] = kindString
		return raw
	}
	if kind, ok := t.kinds[key]; ok {
		switch kind {
		case kindString:
			return raw
		case kindBool:
			if v, ok := parseBool(raw); ok {
				return v
			}
		case kindInt:
			if v, ok := parseInt(raw); ok {
				return v
			}
		case kindFloat:
			if v, ok := parseFloat(raw); ok {
				return v
			}
		case kindNull:
			if isNull(raw) {
				return nil
			}
		}
	}

	if isNull(raw) {
		t.kinds[key] = kindNull
		return nil
	}
	if v, ok := parseBool(raw); ok {
		t.kinds[key] = kindBool
		return v
	}
	if v, ok := parseInt(raw); ok {
		t.kinds[key] = kindInt
		return v
	}
	if v, ok := parseFloat(raw); ok {
		t.kinds[key] = kindFloat
		return v
	}
	t.kinds[key] = kindString
	return raw
}

func parseBool(raw string) (bool, bool) {
	switch strings.ToLower(raw) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func parseInt(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	if raw[0] == '+' || raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9') {
		if !looksFloat(raw) {
			if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

func parseFloat(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return v, true
	}
	return 0, false
}

func looksFloat(raw string) bool {
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '.', 'e', 'E':
			return true
		}
	}
	return false
}

func isNull(raw string) bool {
	switch strings.ToLower(raw) {
	case "nil", "null":
		return true
	default:
		return false
	}
}

type selectorFilter struct {
	selector lql.Selector
	enabled  bool
}

func newSelectorFilter(exprs []string, orMode bool) (selectorFilter, error) {
	cleaned := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		cleaned = append(cleaned, expr)
	}
	if len(cleaned) == 0 {
		return selectorFilter{enabled: false}, nil
	}

	var combined lql.Selector
	for _, expr := range cleaned {
		sel, err := parseSelectorExpr(expr)
		if err != nil {
			return selectorFilter{}, err
		}
		if sel.IsEmpty() {
			continue
		}
		if orMode {
			combined.Or = append(combined.Or, sel)
		} else {
			combined.And = append(combined.And, sel)
		}
	}
	if combined.IsEmpty() {
		return selectorFilter{enabled: false}, nil
	}
	return selectorFilter{selector: combined, enabled: true}, nil
}

func (f selectorFilter) Matches(doc map[string]any) bool {
	if !f.enabled {
		return true
	}
	return lql.Matches(f.selector, doc)
}

func parseSelectorExpr(expr string) (lql.Selector, error) {
	sel, err := lql.ParseSelectorString(expr)
	if err == nil {
		return sel, nil
	}
	parts, splitErr := splitSelectorExpressions(expr)
	if splitErr != nil || len(parts) < 2 {
		return lql.Selector{}, err
	}
	allOr := true
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "or.") && !strings.HasPrefix(trimmed, "or") {
			allOr = false
			break
		}
	}
	if !allOr {
		return lql.Selector{}, err
	}
	orSel, parseErr := lql.ParseSelectorStringsOr(parts)
	if parseErr != nil {
		return lql.Selector{}, err
	}
	return orSel, nil
}

func splitSelectorExpressions(input string) ([]string, error) {
	var expressions []string
	var chunk strings.Builder
	depth := 0
	inQuotes := false
	escape := false
	flush := func() {
		if chunk.Len() == 0 {
			return
		}
		expressions = append(expressions, strings.TrimSpace(chunk.String()))
		chunk.Reset()
	}
	for _, r := range input {
		switch {
		case escape:
			chunk.WriteRune(r)
			escape = false
		case r == '\\':
			escape = true
			chunk.WriteRune(r)
		case r == '"':
			inQuotes = !inQuotes
			chunk.WriteRune(r)
		case r == '{' && !inQuotes:
			depth++
			chunk.WriteRune(r)
		case r == '}' && !inQuotes:
			if depth == 0 {
				return nil, fmt.Errorf("unexpected closing brace in selector expression")
			}
			depth--
			chunk.WriteRune(r)
		case (r == ',' || r == '\n') && !inQuotes && depth == 0:
			flush()
		default:
			chunk.WriteRune(r)
		}
	}
	if inQuotes {
		return nil, fmt.Errorf("unterminated quote in selector expression")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unterminated brace in selector expression")
	}
	flush()
	cleaned := make([]string, 0, len(expressions))
	for _, expr := range expressions {
		if expr != "" {
			cleaned = append(cleaned, expr)
		}
	}
	return cleaned, nil
}

func main() {
	var (
		inputStdin bool
		writeFiles bool
		outDir     string
		orMode     bool
		filters    listFlag
	)
	flag.BoolVar(&inputStdin, "i", false, "read from stdin")
	flag.BoolVar(&writeFiles, "o", false, "write output files instead of stdout")
	flag.StringVar(&outDir, "outdir", "", "output directory when -o is set")
	flag.BoolVar(&orMode, "or", false, "combine -l filters with OR instead of AND")
	flag.Var(&filters, "l", "LQL selector filter (repeatable)")
	flag.Parse()

	args := flag.Args()
	if inputStdin && len(args) > 0 {
		fatalf("cannot combine -i with input files")
	}
	if !inputStdin && len(args) == 0 {
		fatalf("no input files provided (use -i for stdin)")
	}
	if writeFiles && inputStdin {
		fatalf("-o cannot be used with -i (stdin)")
	}
	if outDir != "" && !writeFiles {
		fatalf("-outdir requires -o")
	}
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			fatalf("create outdir: %v", err)
		}
	}

	filter, err := newSelectorFilter(filters, orMode)
	if err != nil {
		fatalf("invalid selector: %v", err)
	}

	now := time.Now()

	if inputStdin {
		if err := processReader(os.Stdin, "stdin", os.Stdout, filter, now); err != nil {
			fatalf("%v", err)
		}
		return
	}

	for _, path := range args {
		out, err := outputWriter(path, writeFiles, outDir)
		if err != nil {
			fatalf("%v", err)
		}
		if err := processFile(path, out, filter, now); err != nil {
			_ = closeOutput(out, writeFiles)
			fatalf("%v", err)
		}
		if err := closeOutput(out, writeFiles); err != nil {
			fatalf("%v", err)
		}
	}
}

func outputWriter(path string, writeFiles bool, outDir string) (io.Writer, error) {
	if !writeFiles {
		return os.Stdout, nil
	}
	outPath := path + ".json"
	if outDir != "" {
		outPath = filepath.Join(outDir, filepath.Base(outPath))
	}
	file, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("open output %s: %w", outPath, err)
	}
	return file, nil
}

func closeOutput(w io.Writer, writeFiles bool) error {
	if !writeFiles {
		return nil
	}
	if c, ok := w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func processFile(path string, out io.Writer, filter selectorFilter, now time.Time) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	return processReader(file, path, out, filter, now)
}

func processReader(r io.Reader, name string, out io.Writer, filter selectorFilter, now time.Time) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	tracker := newTypeTracker()
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := stripANSI(scanner.Text())
		doc, fields, err := parseConsoleLine(line, tracker, now)
		if err != nil {
			warnf("%s:%d: %v", name, lineNo, err)
			continue
		}
		if doc == nil {
			continue
		}
		if !filter.Matches(doc) {
			continue
		}
		if err := writeOrderedJSON(out, doc, fields); err != nil {
			return fmt.Errorf("%s:%d: encode: %w", name, lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: read: %w", name, err)
	}
	return nil
}

func parseConsoleLine(line string, tracker *typeTracker, now time.Time) (map[string]any, []fieldPair, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil, nil
	}
	remaining := line
	doc := make(map[string]any)

	if ts, rest, ok := parseTimestampPrefix(remaining, now); ok {
		doc["ts"] = ts
		remaining = rest
	}

	if lvl, rest, ok := parseLevelPrefix(remaining); ok {
		doc["lvl"] = lvl
		remaining = rest
	}

	remaining = strings.TrimSpace(remaining)
	if remaining == "" {
		return doc, nil, nil
	}

	msg, fieldsPart := splitMessageFields(remaining)
	if msg != "" {
		doc["msg"] = msg
	}
	var fields []fieldPair
	if fieldsPart != "" {
		parsed, err := parseFields(fieldsPart, tracker)
		if err != nil {
			return nil, nil, err
		}
		fields = parsed
		for _, pair := range parsed {
			doc[pair.key] = pair.value
		}
	}
	return doc, fields, nil
}

func parseLevelPrefix(line string) (string, string, bool) {
	line = strings.TrimLeft(line, " ")
	if line == "" {
		return "", line, false
	}
	token, rest := splitToken(line)
	if token == "" {
		return "", line, false
	}
	if lvl, ok := normalizeLevel(token); ok {
		return lvl, strings.TrimLeft(rest, " "), true
	}
	return "", line, false
}

func normalizeLevel(token string) (string, bool) {
	switch strings.ToUpper(token) {
	case "TRC", "TRACE":
		return "trace", true
	case "DBG", "DEBUG":
		return "debug", true
	case "INF", "INFO":
		return "info", true
	case "WRN", "WARN", "WARNING":
		return "warn", true
	case "ERR", "ERROR":
		return "error", true
	case "FTL", "FATAL":
		return "fatal", true
	case "PNC", "PANIC":
		return "panic", true
	case "---", "NOLEVEL", "NO":
		return "nolevel", true
	default:
		return "", false
	}
}

func splitToken(line string) (string, string) {
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' {
			return line[:i], line[i+1:]
		}
	}
	return line, ""
}

func splitMessageFields(line string) (string, string) {
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' {
			continue
		}
		j := i + 1
		for j < len(line) && line[j] == ' ' {
			j++
		}
		if j >= len(line) {
			break
		}
		eq := strings.IndexByte(line[j:], '=')
		if eq == -1 {
			continue
		}
		eq += j
		if eq == j {
			continue
		}
		nextSpace := strings.IndexByte(line[j:], ' ')
		if nextSpace == -1 {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[j:])
		}
		nextSpace += j
		if eq < nextSpace {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[j:])
		}
	}
	return strings.TrimSpace(line), ""
}

type fieldPair struct {
	key   string
	value any
}

func parseFields(line string, tracker *typeTracker) ([]fieldPair, error) {
	fields := make([]fieldPair, 0, 8)
	i := 0
	for i < len(line) {
		for i < len(line) && line[i] == ' ' {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) && line[i] != '=' && line[i] != ' ' {
			i++
		}
		if i >= len(line) || line[i] == ' ' {
			return nil, fmt.Errorf("malformed field tail: missing '=' after %q", line[start:i])
		}
		if line[i] != '=' {
			return nil, fmt.Errorf("malformed field tail near %q", line[start:i])
		}
		key := line[start:i]
		i++
		if key == "" {
			return nil, errors.New("malformed field tail: empty field key")
		}
		if i >= len(line) {
			fields = append(fields, fieldPair{key: key, value: tracker.parseValue(key, "", false)})
			break
		}
		var (
			raw    string
			quoted bool
		)
		if line[i] == '"' {
			value, next, err := parseQuotedValue(line, i)
			if err != nil {
				return nil, err
			}
			raw = value
			quoted = true
			i = next
		} else {
			startVal := i
			for i < len(line) && line[i] != ' ' {
				i++
			}
			raw = line[startVal:i]
			quoted = false
		}
		fields = append(fields, fieldPair{key: key, value: tracker.parseValue(key, raw, quoted)})
	}
	return fields, nil
}

func parseQuotedValue(line string, start int) (string, int, error) {
	if start >= len(line) || line[start] != '"' {
		return "", start, errors.New("invalid quoted value")
	}
	var buf []byte
	for i := start + 1; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			return string(buf), i + 1, nil
		}
		if c != '\\' {
			buf = append(buf, c)
			continue
		}
		if i+1 >= len(line) {
			return "", i, errors.New("unterminated escape")
		}
		next := line[i+1]
		switch next {
		case 'n':
			buf = append(buf, '\n')
			i++
		case 'r':
			buf = append(buf, '\r')
			i++
		case 't':
			buf = append(buf, '\t')
			i++
		case 'b':
			buf = append(buf, '\b')
			i++
		case 'f':
			buf = append(buf, '\f')
			i++
		case '\\':
			buf = append(buf, '\\')
			i++
		case '"':
			buf = append(buf, '"')
			i++
		case 'x':
			if i+3 >= len(line) {
				return "", i, errors.New("short hex escape")
			}
			hi := fromHex(line[i+2])
			lo := fromHex(line[i+3])
			if hi == -1 || lo == -1 {
				return "", i, errors.New("invalid hex escape")
			}
			buf = append(buf, byte(hi<<4|lo))
			i += 3
		default:
			buf = append(buf, next)
			i++
		}
	}
	return "", len(line), errors.New("unterminated quoted value")
}

func fromHex(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	default:
		return -1
	}
}

type layoutSpec struct {
	layout   string
	tokens   int
	hasZone  bool
	yearless bool
}

var timeLayouts = []layoutSpec{
	{layout: time.RFC3339Nano, hasZone: true},
	{layout: time.RFC3339, hasZone: true},
	{layout: "2006-01-02T15:04:05.999999999Z0700", hasZone: true},
	{layout: "2006-01-02T15:04:05Z0700", hasZone: true},
	{layout: "2006-01-02T15:04:05.999999999-0700", hasZone: true},
	{layout: "2006-01-02T15:04:05-0700", hasZone: true},
	{layout: "2006-01-02T15:04:05.999999999", hasZone: false},
	{layout: "2006-01-02T15:04:05", hasZone: false},
	{layout: "2006-01-02 15:04:05.999999999", hasZone: false},
	{layout: "2006-01-02 15:04:05", hasZone: false},
	{layout: "2006/01/02 15:04:05.999999999", hasZone: false},
	{layout: "2006/01/02 15:04:05", hasZone: false},
	{layout: "2006/01/02T15:04:05.999999999", hasZone: false},
	{layout: "2006/01/02T15:04:05", hasZone: false},
	{layout: "2006-01-02", hasZone: false},
	{layout: "2006/01/02", hasZone: false},
	{layout: time.RFC1123Z, hasZone: true},
	{layout: time.RFC1123, hasZone: true},
	{layout: time.RFC822Z, hasZone: true},
	{layout: time.RFC822, hasZone: true},
	{layout: time.RFC850, hasZone: true},
	{layout: time.ANSIC, hasZone: false},
	{layout: time.UnixDate, hasZone: true},
	{layout: time.RubyDate, hasZone: true},
	{layout: "Jan _2 15:04:05", hasZone: false, yearless: true},
}

func init() {
	for i := range timeLayouts {
		timeLayouts[i].tokens = countTokens(timeLayouts[i].layout)
	}
}

func countTokens(layout string) int {
	tokens := 1
	for i := 0; i < len(layout); i++ {
		if layout[i] == ' ' {
			tokens++
		}
	}
	return tokens
}

func parseTimestampPrefix(line string, now time.Time) (string, string, bool) {
	line = strings.TrimLeft(line, " ")
	if line == "" {
		return "", line, false
	}
	token, rest := splitToken(line)
	if token == "" {
		return "", line, false
	}
	if ts, ok := parseEpoch(token); ok {
		return ts, strings.TrimLeft(rest, " "), true
	}
	if ts, ok := parseDTG(token, now); ok {
		return ts, strings.TrimLeft(rest, " "), true
	}

	tokens := tokenize(line)
	for _, spec := range timeLayouts {
		if len(tokens) < spec.tokens {
			continue
		}
		end := tokens[spec.tokens-1].end
		candidate := line[:end]
		var (
			parsed time.Time
			err    error
		)
		if spec.hasZone {
			parsed, err = time.Parse(spec.layout, candidate)
		} else {
			parsed, err = time.ParseInLocation(spec.layout, candidate, time.Local)
		}
		if err != nil {
			continue
		}
		if spec.yearless {
			parsed = time.Date(now.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), parsed.Location())
		}
		return parsed.Format(time.RFC3339Nano), strings.TrimLeft(line[end:], " "), true
	}
	return "", line, false
}

func parseEpoch(token string) (string, bool) {
	if len(token) != 10 && len(token) != 13 && len(token) != 16 && len(token) != 19 {
		return "", false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return "", false
		}
	}
	value, err := strconv.ParseInt(token, 10, 64)
	if err != nil {
		return "", false
	}
	var t time.Time
	switch len(token) {
	case 10:
		t = time.Unix(value, 0)
	case 13:
		t = time.Unix(0, value*int64(time.Millisecond))
	case 16:
		t = time.Unix(0, value*int64(time.Microsecond))
	case 19:
		t = time.Unix(0, value)
	}
	return t.Format(time.RFC3339Nano), true
}

func parseDTG(token string, now time.Time) (string, bool) {
	if len(token) != 6 {
		return "", false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return "", false
		}
	}
	day, _ := strconv.Atoi(token[0:2])
	hour, _ := strconv.Atoi(token[2:4])
	min, _ := strconv.Atoi(token[4:6])
	if day < 1 || hour > 23 || min > 59 {
		return "", false
	}
	location := time.Local
	if now.Location() != nil {
		location = now.Location()
	}
	lastDay := daysInMonth(now.Year(), now.Month(), location)
	if day > lastDay {
		return "", false
	}
	parsed := time.Date(now.Year(), now.Month(), day, hour, min, 0, 0, location)
	return parsed.Format(time.RFC3339Nano), true
}

func daysInMonth(year int, month time.Month, location *time.Location) int {
	if location == nil {
		location = time.Local
	}
	return time.Date(year, month+1, 0, 0, 0, 0, 0, location).Day()
}

type tokenSpan struct {
	start int
	end   int
}

func tokenize(line string) []tokenSpan {
	var tokens []tokenSpan
	i := 0
	for i < len(line) {
		for i < len(line) && line[i] == ' ' {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) && line[i] != ' ' {
			i++
		}
		tokens = append(tokens, tokenSpan{start: start, end: i})
	}
	return tokens
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\x1b' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) || s[i+1] != '[' {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && s[j] != 'm' {
			j++
		}
		if j >= len(s) {
			break
		}
		i = j
	}
	return b.String()
}

func writeOrderedJSON(out io.Writer, doc map[string]any, fields []fieldPair) error {
	var buf bytes.Buffer
	buf.Grow(len(fields) * 16)
	buf.WriteByte('{')
	first := true
	writeKV := func(key string, value any) error {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return err
		}
		valBytes, err := json.Marshal(value)
		if err != nil {
			return err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		buf.Write(valBytes)
		return nil
	}

	if value, ok := doc["ts"]; ok {
		if err := writeKV("ts", value); err != nil {
			return err
		}
	}
	if value, ok := doc["lvl"]; ok {
		if err := writeKV("lvl", value); err != nil {
			return err
		}
	}
	if value, ok := doc["msg"]; ok {
		if err := writeKV("msg", value); err != nil {
			return err
		}
	}
	for _, pair := range fields {
		if err := writeKV(pair.key, pair.value); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	buf.WriteByte('\n')
	_, err := out.Write(buf.Bytes())
	return err
}

func warnf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "pslogconsole2json: %s\n", fmt.Sprintf(format, args...))
}

func fatalf(format string, args ...any) {
	warnf(format, args...)
	os.Exit(2)
}
