package benchmark_test

import (
	"testing"
)

type typedCollector struct {
	strings []stringFieldBench
	ints    []intFieldBench
	bools   []boolFieldBench
	floats  []floatFieldBench
}

type fieldKind uint8

const (
	kindString fieldKind = iota
	kindInt
	kindBool
	kindFloat
)

type flatField struct {
	key  string
	kind fieldKind
	s    string
	i    int64
	b    bool
	f    float64
}

type flatCollector struct {
	fields []flatField
}

func newFlatCollector() *flatCollector {
	return &flatCollector{fields: make([]flatField, 0, 8)}
}

func (c *flatCollector) reset() {
	c.fields = c.fields[:0]
}

func (c *flatCollector) addString(key, val string) {
	c.fields = append(c.fields, flatField{key: key, kind: kindString, s: val})
}

func (c *flatCollector) addInt(key string, val int64) {
	c.fields = append(c.fields, flatField{key: key, kind: kindInt, i: val})
}

func (c *flatCollector) addBool(key string, val bool) {
	c.fields = append(c.fields, flatField{key: key, kind: kindBool, b: val})
}

func (c *flatCollector) addFloat(key string, val float64) {
	c.fields = append(c.fields, flatField{key: key, kind: kindFloat, f: val})
}

func (c *flatCollector) consume() int64 {
	var sum int64
	for _, f := range c.fields {
		sum += dispatchFlat(f)
	}
	return sum
}

type stringFieldBench struct {
	key   string
	value string
}

type intFieldBench struct {
	key   string
	value int64
}

type boolFieldBench struct {
	key   string
	value bool
}

type floatFieldBench struct {
	key   string
	value float64
}

func newTypedCollector() *typedCollector {
	return &typedCollector{
		strings: make([]stringFieldBench, 0, 8),
		ints:    make([]intFieldBench, 0, 8),
		bools:   make([]boolFieldBench, 0, 4),
		floats:  make([]floatFieldBench, 0, 4),
	}
}

func (c *typedCollector) reset() {
	c.strings = c.strings[:0]
	c.ints = c.ints[:0]
	c.bools = c.bools[:0]
	c.floats = c.floats[:0]
}

func (c *typedCollector) addString(key, val string) {
	c.strings = append(c.strings, stringFieldBench{key: key, value: val})
}

func (c *typedCollector) addInt(key string, val int64) {
	c.ints = append(c.ints, intFieldBench{key: key, value: val})
}

func (c *typedCollector) addBool(key string, val bool) {
	c.bools = append(c.bools, boolFieldBench{key: key, value: val})
}

func (c *typedCollector) addFloat(key string, val float64) {
	c.floats = append(c.floats, floatFieldBench{key: key, value: val})
}

func (c *typedCollector) consume() int64 {
	var sum int64
	for _, f := range c.strings {
		sum += int64(len(f.value))
	}
	for _, f := range c.ints {
		sum += f.value
	}
	for _, f := range c.bools {
		if f.value {
			sum++
		}
	}
	for _, f := range c.floats {
		sum += int64(f.value)
	}
	return sum
}

var (
	typedSink int64
	anySink   int64
)

var collectorSampleEntry = func() productionEntry {
	entries, err := loadEmbeddedProductionDataset(1)
	if err != nil {
		panic(err)
	}
	return entries[0]
}()

var collectorAnyPayload = collectorSampleEntry.keyvals

func populateTyped(c *typedCollector) {
	collectorSampleEntry.forEachField(func(key string, value any) {
		switch v := value.(type) {
		case string:
			c.addString(key, v)
		case bool:
			c.addBool(key, v)
		case int:
			c.addInt(key, int64(v))
		case int64:
			c.addInt(key, v)
		case uint64:
			c.addInt(key, int64(v))
		case float64:
			c.addFloat(key, v)
		}
	})
}

func populateFlat(c *flatCollector) {
	collectorSampleEntry.forEachField(func(key string, value any) {
		switch v := value.(type) {
		case string:
			c.addString(key, v)
		case bool:
			c.addBool(key, v)
		case int:
			c.addInt(key, int64(v))
		case int64:
			c.addInt(key, v)
		case uint64:
			c.addInt(key, int64(v))
		case float64:
			c.addFloat(key, v)
		}
	})
}

func fillAny(buf *[16]any) []any {
	keyvals := collectorAnyPayload
	copyLen := len(keyvals)
	if copyLen > len(buf) {
		copyLen = len(buf)
	}
	copy(buf[:copyLen], keyvals[:copyLen])
	return buf[:copyLen]
}

func BenchmarkCollectorPaths(b *testing.B) {
	b.Run("typed", func(b *testing.B) {
		collector := newTypedCollector()
		var sink int64
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			collector.reset()
			populateTyped(collector)
			sink += collector.consume()
		}
		typedSink = sink
	})

	b.Run("flat", func(b *testing.B) {
		collector := newFlatCollector()
		var sink int64
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			collector.reset()
			populateFlat(collector)
			sink += collector.consume()
		}
		typedSink = sink
	})

	b.Run("any/reuse", func(b *testing.B) {
		var sink int64
		payload := collectorAnyPayload
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sink += consumeAny(payload)
		}
		anySink = sink
	})

	b.Run("any/boxing", func(b *testing.B) {
		var sink int64
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var buf [16]any
			keyvals := fillAny(&buf)
			sink += consumeAny(keyvals)
		}
		anySink = sink
	})
}

func consumeAny(payload []any) int64 {
	var sum int64
	for i := 1; i < len(payload); i += 2 {
		switch v := payload[i].(type) {
		case string:
			sum += int64(len(v))
		case int:
			sum += int64(v)
		case int64:
			sum += v
		case bool:
			if v {
				sum++
			}
		case float64:
			sum += int64(v)
		default:
			sum++
		}
	}
	return sum
}

func dispatchFlat(f flatField) int64 {
	switch f.kind {
	case kindString:
		return int64(len(f.s))
	case kindInt:
		return f.i
	case kindBool:
		if f.b {
			return 1
		}
		return 0
	case kindFloat:
		return int64(f.f)
	default:
		return 0
	}
}
