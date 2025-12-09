package pslog

import "testing"

// The helper functions are marked noinline to keep their stack frames visible to
// runtime.Caller during the test.

//go:noinline
func currentFnHelper() string {
	return CurrentFn()
}

//go:noinline
func currentFnInner() string {
	return CurrentFn()
}

//go:noinline
func currentFnOuter() string {
	return currentFnInner()
}

type currentFnReceiver struct{}

//go:noinline
func (currentFnReceiver) ValueMethod() string {
	return CurrentFn()
}

//go:noinline
func (*currentFnReceiver) PointerMethod() string {
	return CurrentFn()
}

func TestCurrentFnReturnsSimpleName(t *testing.T) {
	if got, want := currentFnHelper(), "currentFnHelper"; got != want {
		t.Fatalf("CurrentFn returned %q, want %q", got, want)
	}
}

func TestCurrentFnUsesImmediateCaller(t *testing.T) {
	if got, want := currentFnOuter(), "currentFnInner"; got != want {
		t.Fatalf("CurrentFn should report the direct caller; got %q, want %q", got, want)
	}
}

func TestCurrentFnStripsReceiverAndPackage(t *testing.T) {
	recv := currentFnReceiver{}

	if got, want := recv.ValueMethod(), "ValueMethod"; got != want {
		t.Fatalf("CurrentFn value receiver mismatch: got %q, want %q", got, want)
	}

	if got, want := recv.PointerMethod(), "PointerMethod"; got != want {
		t.Fatalf("CurrentFn pointer receiver mismatch: got %q, want %q", got, want)
	}
}
