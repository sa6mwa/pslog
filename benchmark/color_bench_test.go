package benchmark_test

import (
	"testing"

	pslog "pkt.systems/pslog"
)

var benchKVs = []any{
	"request_id", "req-19f92c0b",
	"user_id", 1337,
	"org_id", 8842,
	"feature", "payments",
	"success", true,
	"retries", 1,
	"latency_ms", 12.74,
	"amount", 199.95,
}

func BenchmarkStructuredColorHotPath(b *testing.B) {
	sink := newBenchmarkSink()
	plain := pslog.NewWithOptions(sink, pslog.Options{Mode: pslog.ModeStructured, TimeFormat: pslog.DTGTimeFormat, MinLevel: pslog.TraceLevel})
	color := pslog.NewWithOptions(sink, pslog.Options{Mode: pslog.ModeStructured, TimeFormat: pslog.DTGTimeFormat, ForceColor: true, MinLevel: pslog.TraceLevel})

	b.Run("plain", func(b *testing.B) {
		sink.resetCount()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			plain.Info("msg", benchKVs...)
		}
		b.ReportMetric(float64(sink.bytesWritten())/float64(b.N), "bytes/op")
	})

	b.Run("color", func(b *testing.B) {
		sink.resetCount()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			color.Info("msg", benchKVs...)
		}
		b.ReportMetric(float64(sink.bytesWritten())/float64(b.N), "bytes/op")
	})
}

func BenchmarkConsoleColorHotPath(b *testing.B) {
	sink := newBenchmarkSink()
	plain := pslog.NewWithOptions(sink, pslog.Options{Mode: pslog.ModeConsole, TimeFormat: pslog.DTGTimeFormat, NoColor: true, MinLevel: pslog.TraceLevel})
	color := pslog.NewWithOptions(sink, pslog.Options{Mode: pslog.ModeConsole, TimeFormat: pslog.DTGTimeFormat, ForceColor: true, MinLevel: pslog.TraceLevel})

	b.Run("plain", func(b *testing.B) {
		sink.resetCount()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			plain.Info("msg", benchKVs...)
		}
		b.ReportMetric(float64(sink.bytesWritten())/float64(b.N), "bytes/op")
	})

	b.Run("color", func(b *testing.B) {
		sink.resetCount()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			color.Info("msg", benchKVs...)
		}
		b.ReportMetric(float64(sink.bytesWritten())/float64(b.N), "bytes/op")
	})
}
