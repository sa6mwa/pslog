package benchmark_test

import (
	"io"
	"testing"

	pslog "pkt.systems/pslog"
)

func BenchmarkPSLogProductionObservedAB(b *testing.B) {
	entries := loadProductionEntries()
	if len(entries) == 0 {
		b.Fatal("no production entries loaded")
	}

	withArgs, staticKeySet := productionStaticWithArgs(entries)
	if len(withArgs) > 0 {
		withArgs = pslog.Keyvals(withArgs...)
	}
	dynamicEntries := entries
	if len(staticKeySet) > 0 {
		dynamicEntries = productionEntriesWithout(entries, staticKeySet)
	}
	keyvalsEntries := productionEntriesWithKeyvals(dynamicEntries)

	type variant struct {
		name       string
		opts       pslog.Options
		useWith    bool
		useKeyvals bool
	}

	variants := []variant{
		{name: "json", opts: pslog.Options{Mode: pslog.ModeStructured}},
		{name: "json+with", opts: pslog.Options{Mode: pslog.ModeStructured}, useWith: true},
		{name: "json+keyvals", opts: pslog.Options{Mode: pslog.ModeStructured}, useWith: true, useKeyvals: true},
		{name: "jsoncolor", opts: pslog.Options{Mode: pslog.ModeStructured, ForceColor: true}},
		{name: "jsoncolor+with", opts: pslog.Options{Mode: pslog.ModeStructured, ForceColor: true}, useWith: true},
		{name: "jsoncolor+keyvals", opts: pslog.Options{Mode: pslog.ModeStructured, ForceColor: true}, useWith: true, useKeyvals: true},
		{name: "console", opts: pslog.Options{Mode: pslog.ModeConsole, NoColor: true}},
		{name: "consolecolor", opts: pslog.Options{Mode: pslog.ModeConsole, ForceColor: true}},
	}

	run := func(name string, opts pslog.Options, useWith bool, useKeyvals bool, observed bool) {
		b.Run(name, func(b *testing.B) {
			sink := newBenchmarkSink()
			var out io.Writer = sink
			if observed {
				out = pslog.NewObservedWriter(out, nil)
			}

			opts.MinLevel = pslog.TraceLevel
			logger := pslog.NewWithOptions(out, opts)
			activeEntries := entries

			if useWith {
				if len(withArgs) > 0 {
					logger = logger.With(withArgs...)
				}
				activeEntries = dynamicEntries
			}
			if useKeyvals {
				activeEntries = keyvalsEntries
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entry := activeEntries[i%len(activeEntries)]
				entry.log(logger)
			}
			if sink.bytesWritten() == 0 {
				b.Fatalf("%s wrote zero bytes", name)
			}
			reportBytesPerOp(b, sink)
		})
	}

	for _, v := range variants {
		run("pslog/production/"+v.name, v.opts, v.useWith, v.useKeyvals, false)
		run("pslog/production/"+v.name+"+observed", v.opts, v.useWith, v.useKeyvals, true)
	}
}
