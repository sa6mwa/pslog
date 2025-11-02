package benchmark_test

import (
    "testing"

    pslog "pkt.systems/pslog"
)

func BenchmarkPSLogJSONOnly(b *testing.B) {
    entries := loadProductionEntries()
    if len(entries) == 0 {
        b.Fatal("no production entries loaded")
    }
    sink := newBenchmarkSink()
    logger := pslog.NewWithOptions(sink, pslog.Options{Mode: pslog.ModeStructured, MinLevel: pslog.TraceLevel})

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

