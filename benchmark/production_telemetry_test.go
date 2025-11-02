package benchmark_test

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	pslog "pkt.systems/pslog"
)

type telemetryStats struct {
	fieldKeyCount         map[string]int
	valueTypeCount        map[string]int
	stringValueCount      map[string]int
	arrayElementTypeCount map[string]int
	totalFields           int
	totalArrayElements    int
}

func newTelemetryStats() *telemetryStats {
	return &telemetryStats{
		fieldKeyCount:         make(map[string]int),
		valueTypeCount:        make(map[string]int),
		stringValueCount:      make(map[string]int),
		arrayElementTypeCount: make(map[string]int),
	}
}

func (s *telemetryStats) addEntry(entry productionEntry) {
	for i := 0; i+1 < len(entry.keyvals); i += 2 {
		key, ok := entry.keyvals[i].(string)
		if !ok {
			continue
		}
		s.totalFields++
		s.fieldKeyCount[key]++
		s.addValue(entry.keyvals[i+1])
	}
}

func (s *telemetryStats) addValue(value any) {
	switch v := value.(type) {
	case pslog.TrustedString:
		s.valueTypeCount["TrustedString"]++
		s.addStringValue(string(v))
	case string:
		s.valueTypeCount["string"]++
		s.addStringValue(v)
	case bool:
		s.valueTypeCount["bool"]++
	case int, int8, int16, int32, int64:
		s.valueTypeCount["int"]++
	case uint, uint8, uint16, uint32, uint64, uintptr:
		s.valueTypeCount["uint"]++
	case float32, float64:
		s.valueTypeCount["float"]++
	case []byte:
		s.valueTypeCount["bytes"]++
	case []any:
		s.valueTypeCount["array"]++
		s.addArray(v)
	case map[string]any:
		s.valueTypeCount["object"]++
		s.addObject(v)
	case nil:
		s.valueTypeCount["nil"]++
	default:
		t := reflect.TypeOf(value)
		name := "unknown"
		if t != nil {
			name = t.String()
		}
		s.valueTypeCount[name]++
	}
}

func (s *telemetryStats) addStringValue(val string) {
	if len(val) == 0 {
		s.stringValueCount["len:0"]++
		return
	}
	bucket := fmt.Sprintf("len:%d-%d", bucketStart(len(val)), bucketEnd(len(val)))
	s.stringValueCount[bucket]++
}

func bucketStart(length int) int {
	switch {
	case length <= 4:
		return 0
	case length <= 8:
		return 5
	case length <= 16:
		return 9
	case length <= 32:
		return 17
	case length <= 64:
		return 33
	case length <= 128:
		return 65
	case length <= 256:
		return 129
	default:
		return 257
	}
}

func bucketEnd(length int) int {
	switch {
	case length <= 4:
		return 4
	case length <= 8:
		return 8
	case length <= 16:
		return 16
	case length <= 32:
		return 32
	case length <= 64:
		return 64
	case length <= 128:
		return 128
	case length <= 256:
		return 256
	default:
		return length
	}
}

func (s *telemetryStats) addArray(values []any) {
	s.totalArrayElements += len(values)
	for _, elem := range values {
		switch v := elem.(type) {
		case pslog.TrustedString:
			s.arrayElementTypeCount["TrustedString"]++
			s.addStringValue(string(v))
		case string:
			s.arrayElementTypeCount["string"]++
			s.addStringValue(v)
		case bool:
			s.arrayElementTypeCount["bool"]++
		case int, int8, int16, int32, int64:
			s.arrayElementTypeCount["int"]++
		case uint, uint8, uint16, uint32, uint64, uintptr:
			s.arrayElementTypeCount["uint"]++
		case float32, float64:
			s.arrayElementTypeCount["float"]++
		case []any:
			s.arrayElementTypeCount["array"]++
			s.addArray(v)
		case map[string]any:
			s.arrayElementTypeCount["object"]++
			s.addObject(v)
		case nil:
			s.arrayElementTypeCount["nil"]++
		default:
			t := reflect.TypeOf(v)
			name := "unknown"
			if t != nil {
				name = t.String()
			}
			s.arrayElementTypeCount[name]++
		}
	}
}

func (s *telemetryStats) addObject(values map[string]any) {
	for _, v := range values {
		s.addValue(v)
	}
}

func (s *telemetryStats) summary() string {
	var sb strings.Builder
	sb.WriteString("telemetry summary\n")
	sb.WriteString(fmt.Sprintf("total fields: %d\n", s.totalFields))
	sb.WriteString("value types:\n")
	writeTopN(&sb, s.valueTypeCount, 12)
	if len(s.stringValueCount) > 0 {
		sb.WriteString("string length buckets:\n")
		writeTopN(&sb, s.stringValueCount, 8)
	}
	if len(s.fieldKeyCount) > 0 {
		sb.WriteString("field keys:\n")
		writeTopN(&sb, s.fieldKeyCount, 12)
	}
	if len(s.arrayElementTypeCount) > 0 {
		sb.WriteString(fmt.Sprintf("array elements (total %d):\n", s.totalArrayElements))
		writeTopN(&sb, s.arrayElementTypeCount, 8)
	}
	return sb.String()
}

func writeTopN(sb *strings.Builder, counts map[string]int, n int) {
	type kv struct {
		key   string
		count int
	}
	slice := make([]kv, 0, len(counts))
	for k, c := range counts {
		slice = append(slice, kv{k, c})
	}
	sort.Slice(slice, func(i, j int) bool {
		if slice[i].count == slice[j].count {
			return slice[i].key < slice[j].key
		}
		return slice[i].count > slice[j].count
	})
	if len(slice) > n {
		slice = slice[:n]
	}
	for _, item := range slice {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", item.key, item.count))
	}
}

func TestProductionTelemetry(t *testing.T) {
	if os.Getenv("PSLOG_TELEMETRY") == "" && testing.Short() {
		t.Skip("skipping telemetry in short mode")
	}
	entries, err := loadEmbeddedProductionDataset(0)
	if err != nil {
		t.Fatalf("failed to load dataset: %v", err)
	}
	stats := newTelemetryStats()
	for _, entry := range entries {
		stats.addEntry(entry)
	}
	t.Logf("%s", stats.summary())
}
