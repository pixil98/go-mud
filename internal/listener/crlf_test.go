package listener

import (
	"bytes"
	"io"
	"testing"
)

// fakeConn is a minimal io.ReadWriter backed by two independent buffers
// so tests can stage input for Read and inspect output from Write.
type fakeConn struct {
	in  *bytes.Buffer
	out *bytes.Buffer
}

func newFakeConn(input string) *fakeConn {
	return &fakeConn{
		in:  bytes.NewBufferString(input),
		out: &bytes.Buffer{},
	}
}

func (f *fakeConn) Read(p []byte) (int, error)  { return f.in.Read(p) }
func (f *fakeConn) Write(p []byte) (int, error) { return f.out.Write(p) }

func TestCRLFRead(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"plain ascii":        {"abc", "abc"},
		"crlf to lf":         {"a\r\nb", "a\nb"},
		"lone cr to lf":      {"a\rb", "a\nb"},
		"multiple crlfs":     {"a\r\nb\r\nc", "a\nb\nc"},
		"trailing cr":        {"abc\r", "abc\n"},
		"only crlf":          {"\r\n", "\n"},
		"mixed cr lf crlf":   {"a\r\nb\rc\nd", "a\nb\nc\nd"},
		"empty":              {"", ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rw := newCRLFReadWriter(newFakeConn(tc.input))
			buf := make([]byte, len(tc.input)+8)
			n, err := rw.Read(buf)
			if err != nil && err != io.EOF {
				t.Fatalf("read error: %v", err)
			}
			if got := string(buf[:n]); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCRLFWrite(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"plain ascii":       {"abc", "abc"},
		"lf to crlf":        {"a\nb", "a\r\nb"},
		"multiple lfs":      {"a\nb\nc", "a\r\nb\r\nc"},
		"trailing lf":       {"abc\n", "abc\r\n"},
		"only lf":           {"\n", "\r\n"},
		"empty":             {"", ""},
		"does not touch cr": {"a\rb", "a\rb"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fc := newFakeConn("")
			rw := newCRLFReadWriter(fc)
			n, err := rw.Write([]byte(tc.input))
			if err != nil {
				t.Fatalf("write error: %v", err)
			}
			if n != len(tc.input) {
				t.Errorf("returned n = %d, want %d", n, len(tc.input))
			}
			if got := fc.out.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCRLFWriteBufferReuse exercises many writes on the same connection to
// catch bugs in any per-connection buffer reuse optimization.
func TestCRLFWriteBufferReuse(t *testing.T) {
	fc := newFakeConn("")
	rw := newCRLFReadWriter(fc)

	inputs := []string{"short\n", "a much longer line with several\nembedded\nnewlines\n", "x", "\n"}
	var want bytes.Buffer
	for _, in := range inputs {
		for _, b := range []byte(in) {
			if b == '\n' {
				want.WriteString("\r\n")
			} else {
				want.WriteByte(b)
			}
		}
	}

	for _, in := range inputs {
		if _, err := rw.Write([]byte(in)); err != nil {
			t.Fatalf("write %q: %v", in, err)
		}
	}

	if got := fc.out.String(); got != want.String() {
		t.Errorf("got %q, want %q", got, want.String())
	}
}
