package benchmark_test

import (
	"io"
	"testing"

	pslog "pkt.systems/pslog"
)

func BenchmarkPSLogJSONOnly(b *testing.B) {
	entries := loadProductionEntries()
	if len(entries) == 0 {
		b.Fatal("no production entries loaded")
	}
	sink := newBenchmarkSink()
	logger := pslog.NewWithOptions(nil, sink, pslog.Options{Mode: pslog.ModeStructured, MinLevel: pslog.TraceLevel})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := entries[i%len(entries)]
		entry.log(logger)
	}
	if sink.bytesWritten() == 0 {
		b.Fatalf("logger wrote zero bytes")
	}
	reportBytesPerOp(b, sink)
}

func BenchmarkPSLogJSONObservedWriter(b *testing.B) {
	entries := loadProductionEntries()
	if len(entries) == 0 {
		b.Fatal("no production entries loaded")
	}

	run := func(name string, observed bool) {
		b.Run(name, func(b *testing.B) {
			sink := newBenchmarkSink()
			var out io.Writer = sink
			if observed {
				out = pslog.NewObservedWriter(out, nil)
			}

			logger := pslog.NewWithOptions(nil, out, pslog.Options{
				Mode:     pslog.ModeStructured,
				MinLevel: pslog.TraceLevel,
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entry := entries[i%len(entries)]
				entry.log(logger)
			}

			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}

	run("pslog/production/json", false)
	run("pslog/production/json+observed", true)
}
