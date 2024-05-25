package rewriter

import (
	"bytes"
	"fmt"
	"testing"
)

func TestWriter(t *testing.T) {
	b := &bytes.Buffer{}
	w := New(b)
	for i := 0; i < 2; i++ {
		_, _ = fmt.Fprintln(w, "foo")
	}
	_ = w.Flush()
	want := "foo\nfoo\n"
	if b.String() != want {
		t.Fatalf("want %q, got %q", want, b.String())
	}
}
