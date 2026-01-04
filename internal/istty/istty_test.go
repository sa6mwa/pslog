package istty

import (
	"os"
	"testing"
)

func TestIsTerminal_InvalidFD(t *testing.T) {
	if IsTerminal(-1) {
		t.Fatalf("expected invalid fd to not be a terminal")
	}
}

func TestIsTerminal_Pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	if IsTerminal(int(r.Fd())) {
		t.Fatalf("expected pipe reader to not be a terminal")
	}
	if IsTerminal(int(w.Fd())) {
		t.Fatalf("expected pipe writer to not be a terminal")
	}
}
