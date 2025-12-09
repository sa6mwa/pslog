package pslog

import (
	"runtime"
	"strings"
)

const (
	unknownFunction = "unknown"
	pslogModulePath = "pkt.systems/pslog"
)

// CurrentFn returns the name of the calling function without package path. If
// the caller cannot be determined it returns "unknown".
//
// Example:
//
//	ctx := context.Background()
//	l := pslog.Ctx(ctx).With("fn", pslog.CurrentFn())
//	l.With(fmt.Errorf("kaboom")).Error("Failed to read message on beeper")
func CurrentFn() string {
	return functionNameFromCaller(2)
}

func functionNameFromCaller(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return unknownFunction
	}
	return functionNameForPC(pc)
}

func functionNameForPC(pc uintptr) string {
	if pc == 0 {
		return unknownFunction
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return unknownFunction
	}
	return trimFunctionName(fn.Name())
}

func trimFunctionName(name string) string {
	if name == "" {
		return unknownFunction
	}
	// Remove package path and package prefix.
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return unknownFunction
	}
	return name
}

// callerFunctionName walks the stack and returns the first frame that is not
// within the pslog module. It mirrors CurrentFn's formatting.
func callerFunctionName() string {
	pcs := make([]uintptr, 16)
	// Skip runtime.Callers and callerFunctionName.
	n := runtime.Callers(2, pcs)
	if n == 0 {
		return unknownFunction
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if frame.Function == "" {
			if !more {
				break
			}
			continue
		}
		if strings.HasPrefix(frame.Function, pslogModulePath+".") || strings.HasPrefix(frame.Function, pslogModulePath+"/") {
			if !more {
				break
			}
			continue
		}
		return trimFunctionName(frame.Function)
	}
	return unknownFunction
}
