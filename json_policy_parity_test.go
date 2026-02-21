package pslog

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestJSONEmitParityVariants(t *testing.T) {
	scenarios := []scenario{
		{
			name:    "runtime_fast",
			keyvals: []any{"user", "alice", "attempts", 3, "latency", 12.5, "ok", true},
		},
		{
			name:     "static_runtime_fast",
			withBase: true,
			keyvals:  []any{"runtime_nan", math.NaN(), "runtime_pos_inf", math.Inf(1), "runtime_neg_inf", math.Inf(-1)},
		},
		{
			name:         "static_runtime_fast_loglevel",
			withBase:     true,
			withLogLevel: true,
			keyvals:      []any{"payload", map[string]any{"nested": "value"}, "status", "ok"},
		},
	}

	policies := []NonFiniteFloatPolicy{
		NonFiniteFloatAsString,
		NonFiniteFloatAsNull,
	}

	for _, policy := range policies {
		for _, sc := range scenarios {
			name := sc.name + "_" + nonFinitePolicyName(policy)
			t.Run(name, func(t *testing.T) {
				plain := emitJSONForParity(t, Options{
					Mode:                 ModeStructured,
					NoColor:              true,
					DisableTimestamp:     true,
					MinLevel:             TraceLevel,
					NonFiniteFloatPolicy: policy,
				}, sc)
				color := emitJSONForParity(t, Options{
					Mode:                 ModeStructured,
					ForceColor:           true,
					DisableTimestamp:     true,
					MinLevel:             TraceLevel,
					NonFiniteFloatPolicy: policy,
				}, sc)
				if !reflect.DeepEqual(plain, color) {
					t.Fatalf("plain/color mismatch\nplain=%v\ncolor=%v", plain, color)
				}
			})
		}
	}
}

func TestJSONEmitParitySlowPath(t *testing.T) {
	sc := scenario{
		name:         "runtime_slow",
		withBase:     true,
		withLogLevel: true,
		keyvals:      []any{"first", "ok", 123, "value", "tail", true},
	}
	plain := emitJSONForParity(t, Options{
		Mode:             ModeStructured,
		NoColor:          true,
		DisableTimestamp: true,
		MinLevel:         TraceLevel,
	}, sc)
	color := emitJSONForParity(t, Options{
		Mode:             ModeStructured,
		ForceColor:       true,
		DisableTimestamp: true,
		MinLevel:         TraceLevel,
	}, sc)
	if !reflect.DeepEqual(plain, color) {
		t.Fatalf("plain/color mismatch\nplain=%v\ncolor=%v", plain, color)
	}
}

func TestJSONFloatPolicyDefaultString(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "plain_default",
			opts: Options{Mode: ModeStructured, NoColor: true, DisableTimestamp: true},
		},
		{
			name: "plain_invalid_falls_back",
			opts: Options{
				Mode:                 ModeStructured,
				NoColor:              true,
				DisableTimestamp:     true,
				NonFiniteFloatPolicy: NonFiniteFloatPolicy(255),
			},
		},
		{
			name: "color_default",
			opts: Options{Mode: ModeStructured, ForceColor: true, DisableTimestamp: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithOptions(nil, &buf, tc.opts).With(
				"base_nan", math.NaN(),
				"base_pos_inf", math.Inf(1),
				"base_neg_inf", math.Inf(-1),
			)
			logger.Info("nf", "nan", math.NaN(), "pos_inf", math.Inf(1), "neg_inf", math.Inf(-1))

			got := decodeJSONLine(t, buf.String(), tc.opts.ForceColor)
			assertJSONString(t, got, "base_nan", "NaN")
			assertJSONString(t, got, "base_pos_inf", "+Inf")
			assertJSONString(t, got, "base_neg_inf", "-Inf")
			assertJSONString(t, got, "nan", "NaN")
			assertJSONString(t, got, "pos_inf", "+Inf")
			assertJSONString(t, got, "neg_inf", "-Inf")
		})
	}
}

func TestJSONFloatPolicyNull(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "plain",
			opts: Options{
				Mode:                 ModeStructured,
				NoColor:              true,
				DisableTimestamp:     true,
				NonFiniteFloatPolicy: NonFiniteFloatAsNull,
			},
		},
		{
			name: "color",
			opts: Options{
				Mode:                 ModeStructured,
				ForceColor:           true,
				DisableTimestamp:     true,
				NonFiniteFloatPolicy: NonFiniteFloatAsNull,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithOptions(nil, &buf, tc.opts).With(
				"base_nan", math.NaN(),
				"base_pos_inf", math.Inf(1),
				"base_neg_inf", math.Inf(-1),
			)
			logger.Info("nf", "nan", math.NaN(), "pos_inf", math.Inf(1), "neg_inf", math.Inf(-1))

			got := decodeJSONLine(t, buf.String(), tc.opts.ForceColor)
			assertJSONNull(t, got, "base_nan")
			assertJSONNull(t, got, "base_pos_inf")
			assertJSONNull(t, got, "base_neg_inf")
			assertJSONNull(t, got, "nan")
			assertJSONNull(t, got, "pos_inf")
			assertJSONNull(t, got, "neg_inf")
		})
	}
}

type scenario struct {
	name         string
	withBase     bool
	withLogLevel bool
	keyvals      []any
}

func emitJSONForParity(t *testing.T, opts Options, sc scenario) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	logger := NewWithOptions(nil, &buf, opts)
	if sc.withBase {
		logger = logger.With(
			"service", "checkout",
			"base_nan", math.NaN(),
			"base_pos_inf", math.Inf(1),
			"base_neg_inf", math.Inf(-1),
		)
	}
	if sc.withLogLevel {
		logger = logger.WithLogLevel()
	}
	logger.Log(InfoLevel, "hello", sc.keyvals...)
	return decodeJSONLine(t, buf.String(), opts.ForceColor)
}

func decodeJSONLine(t *testing.T, raw string, stripANSI bool) map[string]any {
	t.Helper()
	line := strings.TrimSpace(raw)
	if stripANSI {
		line = strings.TrimSpace(stripANSIString(line))
	}
	if !json.Valid([]byte(line)) {
		t.Fatalf("invalid json output: %q", line)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("unmarshal failed: %v (%q)", err, line)
	}
	return got
}

func assertJSONString(t *testing.T, got map[string]any, key, want string) {
	t.Helper()
	value, ok := got[key].(string)
	if !ok || value != want {
		t.Fatalf("expected %s=%q, got %v (%T)", key, want, got[key], got[key])
	}
}

func assertJSONNull(t *testing.T, got map[string]any, key string) {
	t.Helper()
	value, ok := got[key]
	if !ok {
		t.Fatalf("expected key %q in payload", key)
	}
	if value != nil {
		t.Fatalf("expected %s=null, got %v (%T)", key, value, value)
	}
}

func nonFinitePolicyName(policy NonFiniteFloatPolicy) string {
	if policy == NonFiniteFloatAsNull {
		return "null"
	}
	return "string"
}
