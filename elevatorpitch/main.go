package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	plog "github.com/phuslu/log"
	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	pslog "pkt.systems/pslog"
	"pkt.systems/pslog/ansi"
)

const (
	defaultDuration       = 5 * time.Second
	defaultUpdateInterval = 100 * time.Millisecond
	initialBlockSize      = uint64(5000)
	maxChartRows          = 20
	barCell               = "████"
	columnWidth           = 16
	pslogJSONColumn       = 2
	barWidth              = 7
	axisWidth             = 8
	performanceLogName    = "performance.log"
	maxLines              = 13_000_000
)

var (
	blockGlyphs        = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	barLeftPad         = (columnWidth - barWidth) / 2
	barRightPad        = columnWidth - barWidth - barLeftPad
	gradientColors     []string
	diagramPalette     []diagramColor
	loggerOrder        []string
	useGradient        bool
	gradientHorizontal bool
)

type diagramColor struct {
	Bar      string
	Label    string
	RankBold string
}

type runnerMetrics struct {
	name  string
	lines atomic.Uint64
	bytes atomic.Uint64
}

type countingWriter struct {
	mu      sync.Mutex
	metrics *runnerMetrics
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.mu.Lock()
	w.metrics.bytes.Add(uint64(n))
	w.mu.Unlock()
	return n, nil
}

func (w *countingWriter) Sync() error {
	return nil
}

type runner struct {
	name string
	run  func(ctx context.Context, entries []productionEntry, metrics *runnerMetrics)
}

func main() {
	duration := flag.Duration("duration", defaultDuration, "per-logger duration to measure")
	updateInterval := flag.Duration("interval", defaultUpdateInterval, "UI refresh interval")
	limit := flag.Int("limit", 0, "limit number of log entries to replay (0 = all)")
	maxLinesCap := flag.Uint64("max-lines", maxLines, "cap for chart Y axis (lines per run)")
	gradientPaletteName := flag.String("gradient-palette", "blueshift", "gradient palette (synthwave84|outrun|tokyo|solarized|nord|gruvbox|monokai|dracula|catppuccin|gruvboxlight|material|everforest|one-dark|olive|blueshift|grayscale)")
	gradientEnabled := flag.Bool("gradient", true, "use animated gradient for pslog(json)")
	gradientAxis := flag.String("gradient-axis", "horizontal", "gradient axis (horizontal|vertical)")
	diagramPaletteName := flag.String("diagram-palette", "solarized", "diagram palette (monokai|synthwave84|solarized|catppuccin|dracula|gruvboxlight|gruvbox|tokyo|outrun|nord|material|everforest|one-dark)")
	recordFlag := flag.Bool("record", false, "record performance data before playback")
	replayFlag := flag.Bool("replay", false, "replay previously recorded performance data")
	flag.Parse()

	entries, err := loadProductionEntries(*limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load production dataset: %v\n", err)
		os.Exit(1)
	}
	gradientColors = paletteFor(*gradientPaletteName)
	diagramPalette = diagramPaletteFor(*diagramPaletteName)
	useGradient = *gradientEnabled && len(gradientColors) > 0
	gradientHorizontal = strings.ToLower(*gradientAxis) != "vertical"

	pslogJSONName := "pslog" + ansi.Faint + "(json)" + ansi.Reset
	pslogJSONColorName := "pslog" + ansi.Faint + "(jsoncolor)" + ansi.Reset

	loggerOrder = []string{"zerolog", "phuslu", pslogJSONName, pslogJSONColorName, "zap"}

	runners := buildRunners(pslogJSONName, pslogJSONColorName)
	recordRequested := *recordFlag
	replayRequested := *replayFlag

	switch {
	case !recordRequested && !replayRequested:
		runLiveBenchmark(entries, runners, loggerOrder, *duration, *updateInterval, *maxLinesCap)
		return
	}

	var recording performanceRecording
	if recordRequested {
		msg := "Recording performance for %s per logger..."
		fmt.Printf(msg+"\n", formatDuration(*duration))
		fmt.Printf("\033[1F\033[0G")
		fmt.Printf(msg, formatDuration(*duration))
		fmt.Printf("\0337")

		var err error
		recording, err = recordPerformance(entries, runners, *updateInterval, *duration)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to record performance: %v\n", err)
			os.Exit(1)
		}
		if err := savePerformanceLog(performanceLogName, recording); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save %s: %v\n", performanceLogName, err)
			os.Exit(1)
		}
		fmt.Printf("\0338\r")
		fmt.Printf("Saved recording to %s\n", performanceLogName)
	}

	if replayRequested {
		if !recordRequested {
			var err error
			recording, err = loadPerformanceLog(performanceLogName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to load %s: %v\n", performanceLogName, err)
				os.Exit(1)
			}
		}
		runReplay(recording, loggerOrder, *updateInterval, *duration, *maxLinesCap)
		return
	}
}

func buildRunners(pslogJSONName, pslogJSONColorName string) []runner {
	return []runner{
		{
			name: "zerolog",
			run: func(ctx context.Context, entries []productionEntry, metrics *runnerMetrics) {
				writer := &countingWriter{metrics: metrics}
				prevFormat := zerolog.TimeFieldFormat
				prevLevelField := zerolog.LevelFieldName
				prevTimestampField := zerolog.TimestampFieldName
				prevMessageField := zerolog.MessageFieldName
				zerolog.TimeFieldFormat = time.RFC3339
				zerolog.LevelFieldName = "lvl"
				zerolog.TimestampFieldName = "ts"
				zerolog.MessageFieldName = "msg"
				defer func() {
					zerolog.TimeFieldFormat = prevFormat
					zerolog.LevelFieldName = prevLevelField
					zerolog.TimestampFieldName = prevTimestampField
					zerolog.MessageFieldName = prevMessageField
				}()

				logger := zerolog.New(writer).With().Timestamp().Logger().Level(zerolog.TraceLevel)
				replayEntries(ctx, entries, metrics, func(entry productionEntry) {
					lvl := zerologLevelFromPSLog(entry.level)
					ev := logger.WithLevel(lvl)
					entry.applyZerolog(ev).Msg(entry.message)
				})
			},
		},
		{
			name: "phuslu",
			run: func(ctx context.Context, entries []productionEntry, metrics *runnerMetrics) {
				writer := &countingWriter{metrics: metrics}
				logger := &plog.Logger{
					Level:     plog.TraceLevel,
					TimeField: "ts",
					Writer:    plog.IOWriter{Writer: writer},
				}
				logger.TimeFormat = time.RFC3339
				replayEntries(ctx, entries, metrics, func(entry productionEntry) {
					pe := phusluEntryForLevel(logger, entry.level)
					entry.applyPhuslu(pe)
					pe.Msg(entry.message)
				})
			},
		},
		{
			name: pslogJSONName,
			run: func(ctx context.Context, entries []productionEntry, metrics *runnerMetrics) {
				writer := &countingWriter{metrics: metrics}
				opts := pslog.Options{
					Mode:       pslog.ModeStructured,
					NoColor:    true,
					MinLevel:   pslog.TraceLevel,
					TimeFormat: time.RFC3339,
				}
				logger := pslog.NewWithOptions(writer, opts)
				replayEntries(ctx, entries, metrics, func(entry productionEntry) {
					entry.logPslog(logger)
				})
			},
		},
		{
			name: pslogJSONColorName,
			run: func(ctx context.Context, entries []productionEntry, metrics *runnerMetrics) {
				writer := &countingWriter{metrics: metrics}
				opts := pslog.Options{
					Mode:       pslog.ModeStructured,
					ForceColor: true,
					MinLevel:   pslog.TraceLevel,
					TimeFormat: time.RFC3339,
				}
				logger := pslog.NewWithOptions(writer, opts)
				replayEntries(ctx, entries, metrics, func(entry productionEntry) {
					entry.logPslog(logger)
				})
			},
		},
		{
			name: "zap",
			run: func(ctx context.Context, entries []productionEntry, metrics *runnerMetrics) {
				writer := &countingWriter{metrics: metrics}
				encoderCfg := zap.NewProductionEncoderConfig()
				encoderCfg.TimeKey = "ts"
				encoderCfg.MessageKey = "msg"
				encoderCfg.LevelKey = "lvl"
				encoderCfg.CallerKey = ""
				encoderCfg.StacktraceKey = ""
				encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
				encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

				core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(writer), zapcore.DebugLevel)
				logger := zap.New(core, zap.WithCaller(false))
				replayEntries(ctx, entries, metrics, func(entry productionEntry) {
					lvl := zapLevelFromPSLog(entry.level)
					if ce := logger.Check(lvl, entry.message); ce != nil {
						ce.Write(entry.zapFieldsSlice()...)
					}
				})
				_ = logger.Sync()
			},
		},
	}
}

func runLiveBenchmark(entries []productionEntry, runners []runner, order []string, runDuration, updateInterval time.Duration, maxLinesCap uint64) {
	metrics, _ := newMetrics(order)

	ctx, cancel := context.WithTimeout(context.Background(), runDuration)
	defer cancel()

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	start := time.Now()

	var wg sync.WaitGroup
	for i, r := range runners {
		wg.Add(1)
		go func(run runner, m *runnerMetrics) {
			defer wg.Done()
			run.run(ctx, entries, m)
		}(r, metrics[i])
	}

	linesPrinted := renderLoop(ctx, metrics, updateInterval, runDuration, runDuration, start, maxLinesCap)
	cancel()
	wg.Wait()

	finalizeDisplay(metrics, runDuration, runDuration, maxLinesCap, linesPrinted)
}

func runReplay(recording performanceRecording, baseOrder []string, defaultInterval, defaultDuration time.Duration, maxLinesCap uint64) {
	interval := time.Duration(recording.IntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = defaultInterval
	}
	runDuration := time.Duration(recording.RunDurationMs) * time.Millisecond
	if runDuration <= 0 {
		runDuration = defaultDuration
	}

	order := append([]string(nil), baseOrder...)
	metrics, metricsMap := newMetrics(order)

	for _, name := range recording.LoggerNames {
		if _, ok := metricsMap[name]; !ok {
			m := &runnerMetrics{name: name}
			metrics = append(metrics, m)
			metricsMap[name] = m
			order = append(order, name)
		}
	}

	totalDuration := time.Duration(len(recording.Frames)) * interval
	if totalDuration <= 0 {
		totalDuration = runDuration * time.Duration(len(order))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		replayRecording(ctx, cancel, recording, metricsMap, interval)
	}()

	linesPrinted := renderLoop(ctx, metrics, interval, totalDuration, runDuration, start, maxLinesCap)
	cancel()
	wg.Wait()

	finalizeDisplay(metrics, totalDuration, runDuration, maxLinesCap, linesPrinted)
}

func newMetrics(order []string) ([]*runnerMetrics, map[string]*runnerMetrics) {
	metrics := make([]*runnerMetrics, len(order))
	metricsMap := make(map[string]*runnerMetrics, len(order))
	for i, name := range order {
		m := &runnerMetrics{name: name}
		metrics[i] = m
		metricsMap[name] = m
	}
	return metrics, metricsMap
}

func finalizeDisplay(metrics []*runnerMetrics, totalDuration, runDuration time.Duration, maxLinesCap uint64, linesPrinted int) {
	finalBuf, finalLines := buildDisplay(metrics, totalDuration, totalDuration, runDuration, maxLinesCap, true)
	if linesPrinted > 0 {
		fmt.Printf("\033[%dA", linesPrinted)
	}
	os.Stdout.Write(finalBuf.Bytes())
	if finalLines > 0 {
		linesPrinted = finalLines
	}
	fmt.Println()
	printSummary(metrics, runDuration)
}

type performanceSample struct {
	Logger string `json:"logger"`
	Lines  uint64 `json:"lines"`
	Bytes  uint64 `json:"bytes"`
}

type performanceFrame struct {
	Values []performanceSample `json:"values"`
}

type performanceRecording struct {
	IntervalMs    int64              `json:"interval_ms"`
	RunDurationMs int64              `json:"run_duration_ms"`
	LoggerNames   []string           `json:"logger_names"`
	Frames        []performanceFrame `json:"frames"`
}

func recordPerformance(entries []productionEntry, runners []runner, interval, runDuration time.Duration) (performanceRecording, error) {
	rec := performanceRecording{
		IntervalMs:    interval.Milliseconds(),
		RunDurationMs: runDuration.Milliseconds(),
		LoggerNames:   make([]string, len(runners)),
	}
	steps := int(runDuration / interval)
	if steps <= 0 {
		return rec, fmt.Errorf("duration %s too short for interval %s", runDuration, interval)
	}
	rec.Frames = make([]performanceFrame, steps)
	for i := range rec.Frames {
		frame := &rec.Frames[i]
		frame.Values = make([]performanceSample, len(runners))
		for j, r := range runners {
			frame.Values[j].Logger = r.name
		}
	}
	for i, r := range runners {
		rec.LoggerNames[i] = r.name
		metrics := &runnerMetrics{name: r.name}
		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(1)
		go func(run runner) {
			defer wg.Done()
			run.run(ctx, entries, metrics)
		}(r)

		ticker := time.NewTicker(interval)
		for step := 0; step < steps; step++ {
			<-ticker.C
			lines := metrics.lines.Load()
			bytes := metrics.bytes.Load()
			rec.Frames[step].Values[i].Lines = lines
			rec.Frames[step].Values[i].Bytes = bytes
		}
		ticker.Stop()
		cancel()
		wg.Wait()
		// Convert cumulative samples to deltas between frames.
		prevLines := uint64(0)
		prevBytes := uint64(0)
		for step := 0; step < steps; step++ {
			frame := &rec.Frames[step]
			lines := frame.Values[i].Lines
			bytes := frame.Values[i].Bytes
			frame.Values[i].Lines = lines - prevLines
			frame.Values[i].Bytes = bytes - prevBytes
			prevLines = lines
			prevBytes = bytes
		}
	}
	return rec, nil
}

func savePerformanceLog(path string, rec performanceRecording) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(rec)
}

func loadPerformanceLog(path string) (performanceRecording, error) {
	var rec performanceRecording
	f, err := os.Open(path)
	if err != nil {
		return rec, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err := dec.Decode(&rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func replayRecording(ctx context.Context, cancel context.CancelFunc, rec performanceRecording, metrics map[string]*runnerMetrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for idx, frame := range rec.Frames {
		if idx > 0 {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
		for _, sample := range frame.Values {
			if metric, ok := metrics[sample.Logger]; ok {
				if sample.Lines > 0 {
					metric.lines.Add(sample.Lines)
				}
				if sample.Bytes > 0 {
					metric.bytes.Add(sample.Bytes)
				}
			}
		}
	}
	cancel()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func replayEntries(ctx context.Context, entries []productionEntry, metrics *runnerMetrics, emit func(productionEntry)) {
	if len(entries) == 0 {
		return
	}
	index := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		entry := entries[index]
		index++
		if index == len(entries) {
			index = 0
		}
		metrics.lines.Add(1)
		emit(entry)
	}
}

func renderLoop(ctx context.Context, metrics []*runnerMetrics, interval time.Duration, totalDuration time.Duration, runDuration time.Duration, start time.Time, maxLinesCap uint64) int {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var linesPrinted int
	first := true
	for {
		now := time.Now()
		elapsed := min(now.Sub(start), totalDuration)
		buf, lineCount := buildDisplay(metrics, elapsed, totalDuration, runDuration, maxLinesCap, false)
		if first {
			first = false
		} else if linesPrinted > 0 {
			fmt.Printf("\033[%dA", linesPrinted)
		}
		os.Stdout.Write(buf.Bytes())
		linesPrinted = lineCount

		if ctx.Err() != nil {
			return linesPrinted
		}

		select {
		case <-ctx.Done():
			return linesPrinted
		case <-ticker.C:
		}
	}
}

func buildDisplay(metrics []*runnerMetrics, elapsed, totalDuration, runDuration time.Duration, maxLinesCap uint64, showRanks bool) (*bytes.Buffer, int) {
	snapshots := snapshotMetrics(metrics)
	effective := make([]uint64, len(snapshots))
	var maxEffective uint64
	for i, snap := range snapshots {
		value := snap.lines
		if maxLinesCap > 0 && value > maxLinesCap {
			value = maxLinesCap
		}
		effective[i] = value
		if value > maxEffective {
			maxEffective = value
		}
	}

	ranks := computeRanks(snapshots)
	blockValue, height := computeScale(maxEffective, maxLinesCap)

	var buf bytes.Buffer
	lines := 0

	buf.WriteString(colorizeAxis(fmt.Sprintf("Y = loglines (per %s run)       each %s = %s loglines\n", formatDuration(runDuration), barCell, formatNumber(blockValue))))
	lines++
	totalWidth := axisWidth + columnWidth*len(snapshots)
	buf.WriteString(colorizeFrame("┌"))
	buf.WriteString(colorizeFrame(strings.Repeat("─", totalWidth)))
	buf.WriteString(colorizeFrame("┐\n"))
	lines++

	rankRows := make([]int, len(snapshots))
	if showRanks {
		for i, value := range effective {
			if value == 0 {
				continue
			}
			rankRows[i] = int(math.Ceil(float64(value) / float64(blockValue)))
		}
	}

	gradientShift := 0
	if useGradient && len(gradientColors) > 0 {
		gradientShift = int(elapsed / (100 * time.Millisecond))
	}

	for row := height; row >= 1; row-- {
		threshold := uint64(row) * blockValue
		var lower uint64
		if row > 1 {
			lower = uint64(row-1) * blockValue
		}
		label := fmt.Sprintf("%5s ┤", formatNumber(threshold))
		buf.WriteString(colorizeFrame("│"))
		buf.WriteString(colorizeAxis(padRight(label, axisWidth)))
		for i := range snapshots {
			if showRanks && rankRows[i] > 0 && row == rankRows[i]+1 {
				buf.WriteString(colorizeRank(i, centerText(strconv.Itoa(ranks[i]), columnWidth)))
			} else {
				buf.WriteString(renderBarCell(i, row, gradientShift, effective[i], lower, threshold))
			}
		}
		buf.WriteString(colorizeFrame("│\n"))
		lines++
	}

	zeroLabel := fmt.Sprintf("%5s ┼", "0")
	buf.WriteString(colorizeFrame("│"))
	buf.WriteString(colorizeAxis(padRight(zeroLabel, axisWidth)))
	for range snapshots {
		buf.WriteString(colorizeAxis(strings.Repeat("─", columnWidth)))
	}
	buf.WriteString(colorizeFrame("┤\n"))
	lines++

	buf.WriteString(colorizeFrame("│"))
	buf.WriteString(strings.Repeat(" ", axisWidth))
	for i, snap := range snapshots {
		buf.WriteString(colorizeLabel(i, centerText(snap.name, columnWidth)))
	}
	buf.WriteString(colorizeFrame("│\n"))
	lines++

	buf.WriteString(colorizeFrame("│"))
	buf.WriteString(strings.Repeat(" ", axisWidth))
	for i, snap := range snapshots {
		lineText := fmt.Sprintf("%s lines", formatNumber(snap.lines))
		buf.WriteString(colorizeLabel(i, centerText(lineText, columnWidth)))
	}
	buf.WriteString(colorizeFrame("│\n"))
	lines++

	buf.WriteString(colorizeFrame("│"))
	elapsedText := fmt.Sprintf("%s/%s", formatDuration(elapsed), formatDuration(totalDuration))
	buf.WriteString(colorizeAxis(centerText(elapsedText, axisWidth)))
	window := elapsed
	if window <= 0 {
		window = time.Millisecond
	}
	for i, snap := range snapshots {
		perSecond := float64(snap.lines)
		perSecond /= window.Seconds()
		rateText := fmt.Sprintf("%.0f/s", perSecond)
		buf.WriteString(colorizeLabel(i, centerText(rateText, columnWidth)))
	}
	buf.WriteString(colorizeFrame("│\n"))
	lines++

	buf.WriteString(colorizeFrame("└"))
	buf.WriteString(colorizeFrame(strings.Repeat("─", totalWidth)))
	buf.WriteString(colorizeFrame("┘\n"))
	lines++

	return &buf, lines
}

type snapshot struct {
	name  string
	lines uint64
	bytes uint64
}

func snapshotMetrics(metrics []*runnerMetrics) []snapshot {
	out := make([]snapshot, len(metrics))
	for i, m := range metrics {
		out[i] = snapshot{
			name:  m.name,
			lines: m.lines.Load(),
			bytes: m.bytes.Load(),
		}
	}
	return out
}

func computeRanks(snapshots []snapshot) []int {
	type pair struct {
		idx   int
		lines uint64
		name  string
	}
	pairs := make([]pair, len(snapshots))
	for i, snap := range snapshots {
		pairs[i] = pair{idx: i, lines: snap.lines, name: snap.name}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].lines == pairs[j].lines {
			return pairs[i].idx < pairs[j].idx
		}
		return pairs[i].lines > pairs[j].lines
	})
	ranks := make([]int, len(snapshots))
	rank := 1
	for _, p := range pairs {
		ranks[p.idx] = rank
		rank++
	}
	return ranks
}

func computeScale(maxLines, maxCap uint64) (uint64, int) {
	rows := uint64(maxChartRows)
	if rows == 0 {
		rows = 1
	}

	target := maxLines
	if maxCap > 0 {
		target = maxCap
	}
	if target == 0 {
		target = initialBlockSize * rows
	}

	minStep := target / rows
	if target%rows != 0 {
		minStep++
	}
	if minStep == 0 {
		minStep = 1
	}

	block := ((minStep + initialBlockSize - 1) / initialBlockSize) * initialBlockSize
	if block == 0 {
		block = initialBlockSize
	}

	return block, int(rows)
}

func renderBarCell(column, row, shift int, value, lower, upper uint64) string {
	if value <= lower || upper <= lower {
		return strings.Repeat(" ", columnWidth)
	}

	glyph := blockGlyphForFraction(float64(value-lower) / float64(upper-lower))
	glyphStr := string(glyph)
	bar := strings.Repeat(glyphStr, barWidth)
	left := strings.Repeat(" ", barLeftPad)
	right := strings.Repeat(" ", barRightPad)

	if column == pslogJSONColumn && useGradient {
		if gradientHorizontal {
			var builder strings.Builder
			builder.WriteString(left)
			for j := range barWidth {
				color := gradientColorAt(shift + j)
				builder.WriteString(color)
				builder.WriteString(glyphStr)
			}
			builder.WriteString(ansi.Reset)
			builder.WriteString(right)
			return builder.String()
		}
		color := gradientColorAt(shift + row)
		return left + color + bar + ansi.Reset + right
	}
	return left + colorizeBar(column, bar) + right
}

func blockGlyphForFraction(fraction float64) string {
	if fraction <= 0 {
		return " "
	}
	if fraction >= 1 {
		return string(blockGlyphs[len(blockGlyphs)-1])
	}
	steps := float64(len(blockGlyphs))
	index := max(int(math.Ceil(fraction*steps))-1, 0)
	if index >= len(blockGlyphs) {
		index = len(blockGlyphs) - 1
	}
	return string(blockGlyphs[index])
}

func centerText(text string, width int) string {
	if visualWidth(text) >= width {
		return truncateToWidth(text, width)
	}
	padding := width - visualWidth(text)
	left := padding / 2
	right := padding - left
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
}

func visualWidth(s string) int {
	width := 0
	for i := 0; i < len(s); {
		if next, ok := skipANSI(s, i); ok {
			i = next
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		width++
		i += size
	}
	return width
}

func truncateToWidth(s string, max int) string {
	if visualWidth(s) <= max {
		return s
	}
	var builder strings.Builder
	width := 0
	for i := 0; i < len(s); {
		if next, ok := skipANSI(s, i); ok {
			builder.WriteString(s[i:next])
			i = next
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if width+1 > max {
			break
		}
		builder.WriteRune(r)
		width++
		i += size
	}
	return builder.String()
}

func padRight(s string, width int) string {
	current := visualWidth(s)
	if current >= width {
		return truncateToWidth(s, width)
	}
	return s + strings.Repeat(" ", width-current)
}

func skipANSI(s string, i int) (int, bool) {
	if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
		j := i + 2
		for j < len(s) && s[j] != 'm' {
			j++
		}
		if j < len(s) {
			return j + 1, true
		}
	}
	return i, false
}

func formatNumber(value uint64) string {
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	case value >= 10_000:
		return fmt.Sprintf("%.0fk", float64(value)/1_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fk", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func formatDuration(d time.Duration) string {
	seconds := d.Seconds()
	if seconds >= 1 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func printSummary(metrics []*runnerMetrics, runDuration time.Duration) {
	type summaryEntry struct {
		name  string
		lines uint64
	}
	entries := make([]summaryEntry, len(metrics))
	for i, m := range metrics {
		entries[i] = summaryEntry{name: m.name, lines: m.lines.Load()}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].lines == entries[j].lines {
			return entries[i].name < entries[j].name
		}
		return entries[i].lines > entries[j].lines
	})
	fmt.Printf("Results after %s per logger:\n", runDuration.Round(10*time.Millisecond))
	for i, entry := range entries {
		perSecond := float64(entry.lines) / runDuration.Seconds()
		colIdx := loggerColumnIndex(entry.name)
		rank := colorizeRank(colIdx, fmt.Sprintf("%d.", i+1))
		name := colorizeLabel(colIdx, entry.name)
		padding := padSpaces(entry.name, 20)
		fmt.Printf("  %s %s%s%10d lines  (%6.0f lines/s)\n", rank, name, padding, entry.lines, perSecond)
	}
}

func loggerColumnIndex(name string) int {
	for idx, n := range loggerOrder {
		if n == name {
			return idx
		}
	}
	return 0
}

func padSpaces(s string, total int) string {
	width := visualWidth(s)
	if width >= total {
		return ""
	}
	return strings.Repeat(" ", total-width)
}

func gradientColorAt(offset int) string {
	if len(gradientColors) == 0 {
		return ""
	}
	if len(gradientColors) == 1 {
		return gradientColors[0]
	}
	period := 2 * (len(gradientColors) - 1)
	m := offset % period
	if m < 0 {
		m += period
	}
	if m >= len(gradientColors) {
		m = period - m
	}
	return gradientColors[m]
}

func diagramColorFor(column int) diagramColor {
	if len(diagramPalette) == 0 {
		return diagramColor{}
	}
	idx := column % len(diagramPalette)
	return diagramPalette[idx]
}

func colorizeBar(column int, bar string) string {
	dc := diagramColorFor(column)
	if dc.Bar == "" {
		return bar
	}
	return dc.Bar + bar + ansi.Reset
}

func colorizeLabel(column int, text string) string {
	dc := diagramColorFor(column)
	if dc.Label == "" {
		return text
	}
	return dc.Label + text + ansi.Reset
}

func colorizeRank(column int, text string) string {
	dc := diagramColorFor(column)
	if dc.RankBold == "" {
		return text
	}
	return dc.RankBold + text + ansi.Reset
}

func colorizeFrame(text string) string {
	if len(diagramPalette) == 0 || diagramPalette[0].Bar == "" {
		return text
	}
	return diagramPalette[0].Bar + text + ansi.Reset
}

func colorizeAxis(text string) string {
	if len(diagramPalette) == 0 || diagramPalette[0].Label == "" {
		return text
	}
	return diagramPalette[0].Label + text + ansi.Reset
}

func paletteFor(name string) []string {
	switch strings.ToLower(name) {
	case "outrun", "outrun-electric", "outrun_electric":
		return []string{
			"\x1b[38;5;201m",
			"\x1b[38;5;207m",
			"\x1b[38;5;213m",
			"\x1b[38;5;219m",
			"\x1b[38;5;45m",
			"\x1b[38;5;51m",
			"\x1b[38;5;39m",
			"\x1b[38;5;99m",
			"\x1b[38;5;135m",
		}
	case "gruvbox":
		return []string{
			"\x1b[38;5;214m",
			"\x1b[38;5;208m",
			"\x1b[38;5;202m",
			"\x1b[38;5;166m",
			"\x1b[38;5;94m",
			"\x1b[38;5;100m",
			"\x1b[38;5;142m",
			"\x1b[38;5;178m",
		}
	case "tokyo", "tokyo-night", "tokyo_night":
		return []string{
			"\x1b[38;5;69m",
			"\x1b[38;5;75m",
			"\x1b[38;5;81m",
			"\x1b[38;5;177m",
			"\x1b[38;5;213m",
			"\x1b[38;5;219m",
			"\x1b[38;5;207m",
		}
	case "solarized", "solarized-night", "solarized_night":
		return []string{
			"\x1b[38;5;37m",
			"\x1b[38;5;44m",
			"\x1b[38;5;51m",
			"\x1b[38;5;86m",
			"\x1b[38;5;108m",
			"\x1b[38;5;137m",
			"\x1b[38;5;172m",
		}
	case "nord":
		return []string{
			"\x1b[38;5;117m",
			"\x1b[38;5;111m",
			"\x1b[38;5;105m",
			"\x1b[38;5;69m",
			"\x1b[38;5;102m",
			"\x1b[38;5;144m",
			"\x1b[38;5;152m",
		}
	case "monokai":
		return []string{
			"\x1b[38;5;229m",
			"\x1b[38;5;221m",
			"\x1b[38;5;215m",
			"\x1b[38;5;208m",
			"\x1b[38;5;198m",
			"\x1b[38;5;200m",
			"\x1b[38;5;135m",
			"\x1b[38;5;141m",
			"\x1b[38;5;118m",
		}
	case "dracula":
		return []string{
			"\x1b[38;5;219m",
			"\x1b[38;5;213m",
			"\x1b[38;5;207m",
			"\x1b[38;5;201m",
			"\x1b[38;5;99m",
			"\x1b[38;5;63m",
			"\x1b[38;5;104m",
			"\x1b[38;5;111m",
		}
	case "catppuccin", "catppuccin-mocha":
		return []string{
			"\x1b[38;5;217m",
			"\x1b[38;5;223m",
			"\x1b[38;5;189m",
			"\x1b[38;5;152m",
			"\x1b[38;5;110m",
			"\x1b[38;5;147m",
			"\x1b[38;5;183m",
			"\x1b[38;5;150m",
		}
	case "synthwave84", "synthwave", "default":
		fallthrough
	default:
		return []string{
			"\x1b[38;5;201m",
			"\x1b[38;5;207m",
			"\x1b[38;5;213m",
			"\x1b[38;5;219m",
			"\x1b[38;5;45m",
			"\x1b[38;5;51m",
			"\x1b[38;5;39m",
			"\x1b[38;5;99m",
			"\x1b[38;5;135m",
			"\x1b[38;5;207m",
			"\x1b[38;5;147m",
			"\x1b[38;5;111m",
		}
	case "olive", "olivegreen":
		return []string{
			"\x1b[38;5;106m",
			"\x1b[38;5;142m",
			"\x1b[38;5;148m",
			"\x1b[38;5;149m",
			"\x1b[38;5;150m",
			"\x1b[38;5;151m",
			"\x1b[38;5;107m",
			"\x1b[38;5;71m",
		}
	case "blueshift", "blue":
		return []string{
			"\x1b[38;5;17m",
			"\x1b[38;5;18m",
			"\x1b[38;5;19m",
			"\x1b[38;5;20m",
			"\x1b[38;5;25m",
			"\x1b[38;5;26m",
			"\x1b[38;5;32m",
			"\x1b[38;5;38m",
		}
	case "grayscale", "grey", "gray":
		return []string{
			"\x1b[38;5;232m",
			"\x1b[38;5;233m",
			"\x1b[38;5;234m",
			"\x1b[38;5;235m",
			"\x1b[38;5;236m",
			"\x1b[38;5;237m",
			"\x1b[38;5;238m",
			"\x1b[38;5;239m",
			"\x1b[38;5;240m",
			"\x1b[38;5;244m",
			"\x1b[38;5;247m",
			"\x1b[38;5;250m",
		}
	}
}

func diagramPaletteFor(name string) []diagramColor {
	switch strings.ToLower(name) {
	case "solarized", "solarized-nightfall":
		return []diagramColor{
			{Bar: "\x1b[38;5;37m", Label: "\x1b[38;5;109m", RankBold: "\x1b[1;38;5;229m"},
			{Bar: "\x1b[38;5;136m", Label: "\x1b[38;5;178m", RankBold: "\x1b[1;38;5;230m"},
			{Bar: "\x1b[38;5;51m", Label: "\x1b[38;5;44m", RankBold: "\x1b[1;38;5;123m"},
			{Bar: "\x1b[38;5;166m", Label: "\x1b[38;5;208m", RankBold: "\x1b[1;38;5;214m"},
			{Bar: "\x1b[38;5;108m", Label: "\x1b[38;5;65m", RankBold: "\x1b[1;38;5;152m"},
		}
	case "catppuccin", "catppuccin-mocha":
		return []diagramColor{
			{Bar: "\x1b[38;5;217m", Label: "\x1b[38;5;183m", RankBold: "\x1b[1;38;5;223m"},
			{Bar: "\x1b[38;5;147m", Label: "\x1b[38;5;110m", RankBold: "\x1b[1;38;5;152m"},
			{Bar: "\x1b[38;5;109m", Label: "\x1b[38;5;150m", RankBold: "\x1b[1;38;5;189m"},
			{Bar: "\x1b[38;5;216m", Label: "\x1b[38;5;182m", RankBold: "\x1b[1;38;5;223m"},
			{Bar: "\x1b[38;5;205m", Label: "\x1b[38;5;204m", RankBold: "\x1b[1;38;5;218m"},
		}
	case "dracula":
		return []diagramColor{
			{Bar: "\x1b[38;5;219m", Label: "\x1b[38;5;147m", RankBold: "\x1b[1;38;5;225m"},
			{Bar: "\x1b[38;5;81m", Label: "\x1b[38;5;111m", RankBold: "\x1b[1;38;5;117m"},
			{Bar: "\x1b[38;5;204m", Label: "\x1b[38;5;170m", RankBold: "\x1b[1;38;5;218m"},
			{Bar: "\x1b[38;5;141m", Label: "\x1b[38;5;111m", RankBold: "\x1b[1;38;5;189m"},
			{Bar: "\x1b[38;5;177m", Label: "\x1b[38;5;171m", RankBold: "\x1b[1;38;5;183m"},
		}
	case "gruvboxlight", "gruvbox-light":
		return []diagramColor{
			{Bar: "\x1b[38;5;130m", Label: "\x1b[38;5;136m", RankBold: "\x1b[1;38;5;173m"},
			{Bar: "\x1b[38;5;108m", Label: "\x1b[38;5;107m", RankBold: "\x1b[1;38;5;150m"},
			{Bar: "\x1b[38;5;66m", Label: "\x1b[38;5;73m", RankBold: "\x1b[1;38;5;109m"},
			{Bar: "\x1b[38;5;142m", Label: "\x1b[38;5;142m", RankBold: "\x1b[1;38;5;179m"},
			{Bar: "\x1b[38;5;167m", Label: "\x1b[38;5;167m", RankBold: "\x1b[1;38;5;203m"},
		}
	case "gruvbox":
		return []diagramColor{
			{Bar: "\x1b[38;5;214m", Label: "\x1b[38;5;214m", RankBold: "\x1b[1;38;5;208m"},
			{Bar: "\x1b[38;5;142m", Label: "\x1b[38;5;142m", RankBold: "\x1b[1;38;5;190m"},
			{Bar: "\x1b[38;5;108m", Label: "\x1b[38;5;108m", RankBold: "\x1b[1;38;5;150m"},
			{Bar: "\x1b[38;5;172m", Label: "\x1b[38;5;109m", RankBold: "\x1b[1;38;5;214m"},
			{Bar: "\x1b[38;5;167m", Label: "\x1b[38;5;167m", RankBold: "\x1b[1;38;5;203m"},
		}
	case "tokyo", "tokyo-night":
		return []diagramColor{
			{Bar: "\x1b[38;5;69m", Label: "\x1b[38;5;74m", RankBold: "\x1b[1;38;5;111m"},
			{Bar: "\x1b[38;5;110m", Label: "\x1b[38;5;110m", RankBold: "\x1b[1;38;5;218m"},
			{Bar: "\x1b[38;5;176m", Label: "\x1b[38;5;173m", RankBold: "\x1b[1;38;5;219m"},
			{Bar: "\x1b[38;5;117m", Label: "\x1b[38;5;117m", RankBold: "\x1b[1;38;5;123m"},
			{Bar: "\x1b[38;5;210m", Label: "\x1b[38;5;210m", RankBold: "\x1b[1;38;5;213m"},
		}
	case "outrun":
		return []diagramColor{
			{Bar: "\x1b[38;5;201m", Label: "\x1b[38;5;219m", RankBold: "\x1b[1;38;5;225m"},
			{Bar: "\x1b[38;5;51m", Label: "\x1b[38;5;45m", RankBold: "\x1b[1;38;5;51m"},
			{Bar: "\x1b[38;5;99m", Label: "\x1b[38;5;135m", RankBold: "\x1b[1;38;5;201m"},
			{Bar: "\x1b[38;5;207m", Label: "\x1b[38;5;213m", RankBold: "\x1b[1;38;5;219m"},
			{Bar: "\x1b[38;5;39m", Label: "\x1b[38;5;81m", RankBold: "\x1b[1;38;5;45m"},
		}
	case "synthwave84", "synthwave":
		return []diagramColor{
			{Bar: "\x1b[38;5;201m", Label: "\x1b[38;5;213m", RankBold: "\x1b[1;38;5;219m"},
			{Bar: "\x1b[38;5;51m", Label: "\x1b[38;5;45m", RankBold: "\x1b[1;38;5;81m"},
			{Bar: "\x1b[38;5;135m", Label: "\x1b[38;5;135m", RankBold: "\x1b[1;38;5;207m"},
			{Bar: "\x1b[38;5;99m", Label: "\x1b[38;5;99m", RankBold: "\x1b[1;38;5;135m"},
			{Bar: "\x1b[38;5;45m", Label: "\x1b[38;5;51m", RankBold: "\x1b[1;38;5;51m"},
		}
	case "nord":
		return []diagramColor{
			{Bar: "\x1b[38;5;117m", Label: "\x1b[38;5;111m", RankBold: "\x1b[1;38;5;189m"},
			{Bar: "\x1b[38;5;74m", Label: "\x1b[38;5;74m", RankBold: "\x1b[1;38;5;189m"},
			{Bar: "\x1b[38;5;109m", Label: "\x1b[38;5;109m", RankBold: "\x1b[1;38;5;195m"},
			{Bar: "\x1b[38;5;103m", Label: "\x1b[38;5;103m", RankBold: "\x1b[1;38;5;153m"},
			{Bar: "\x1b[38;5;152m", Label: "\x1b[38;5;152m", RankBold: "\x1b[1;38;5;195m"},
		}
	case "material":
		return []diagramColor{
			{Bar: "\x1b[38;5;75m", Label: "\x1b[38;5;75m", RankBold: "\x1b[1;38;5;123m"},
			{Bar: "\x1b[38;5;110m", Label: "\x1b[38;5;110m", RankBold: "\x1b[1;38;5;189m"},
			{Bar: "\x1b[38;5;150m", Label: "\x1b[38;5;150m", RankBold: "\x1b[1;38;5;194m"},
			{Bar: "\x1b[38;5;180m", Label: "\x1b[38;5;180m", RankBold: "\x1b[1;38;5;223m"},
			{Bar: "\x1b[38;5;204m", Label: "\x1b[38;5;204m", RankBold: "\x1b[1;38;5;213m"},
		}
	case "everforest":
		return []diagramColor{
			{Bar: "\x1b[38;5;143m", Label: "\x1b[38;5;143m", RankBold: "\x1b[1;38;5;179m"},
			{Bar: "\x1b[38;5;107m", Label: "\x1b[38;5;107m", RankBold: "\x1b[1;38;5;150m"},
			{Bar: "\x1b[38;5;65m", Label: "\x1b[38;5;65m", RankBold: "\x1b[1;38;5;108m"},
			{Bar: "\x1b[38;5;179m", Label: "\x1b[38;5;179m", RankBold: "\x1b[1;38;5;221m"},
			{Bar: "\x1b[38;5;138m", Label: "\x1b[38;5;138m", RankBold: "\x1b[1;38;5;222m"},
		}
	case "one-dark", "one-dark-pro":
		return []diagramColor{
			{Bar: "\x1b[38;5;80m", Label: "\x1b[38;5;80m", RankBold: "\x1b[1;38;5;123m"},
			{Bar: "\x1b[38;5;110m", Label: "\x1b[38;5;110m", RankBold: "\x1b[1;38;5;189m"},
			{Bar: "\x1b[38;5;180m", Label: "\x1b[38;5;180m", RankBold: "\x1b[1;38;5;223m"},
			{Bar: "\x1b[38;5;173m", Label: "\x1b[38;5;173m", RankBold: "\x1b[1;38;5;219m"},
			{Bar: "\x1b[38;5;202m", Label: "\x1b[38;5;202m", RankBold: "\x1b[1;38;5;208m"},
		}
	case "monokai", "monokai-vibrant", "default":
		fallthrough
	default:
		return []diagramColor{
			{Bar: "\x1b[38;5;229m", Label: "\x1b[38;5;229m", RankBold: "\x1b[1;38;5;229m"},
			{Bar: "\x1b[38;5;141m", Label: "\x1b[38;5;141m", RankBold: "\x1b[1;38;5;189m"},
			{Bar: "\x1b[38;5;118m", Label: "\x1b[38;5;118m", RankBold: "\x1b[1;38;5;118m"},
			{Bar: "\x1b[38;5;198m", Label: "\x1b[38;5;198m", RankBold: "\x1b[1;38;5;205m"},
			{Bar: "\x1b[38;5;215m", Label: "\x1b[38;5;215m", RankBold: "\x1b[1;38;5;221m"},
		}
	}
}
