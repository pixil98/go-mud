package listener

import (
	"io"
)

// crlfWriter wraps an io.ReadWriter and converts \n to \r\n on writes.
// This is needed for protocols like telnet that require CRLF line endings.
//
// Not safe for concurrent Writes; callers are expected to hold the write-side
// of a connection on a single goroutine (as player.Player does).
type crlfWriter struct {
	rw  io.ReadWriter
	buf []byte // reusable write-side buffer
}

func newCRLFReadWriter(rw io.ReadWriter) io.ReadWriter {
	return &crlfWriter{rw: rw}
}

// Read normalizes incoming line endings in place: \r\n → \n and lone \r → \n.
// Telnet sends \r\n; SSH with a PTY often sends just \r.
func (c *crlfWriter) Read(p []byte) (int, error) {
	n, err := c.rw.Read(p)
	if n <= 0 {
		return n, err
	}
	w := 0
	for r := 0; r < n; r++ {
		switch p[r] {
		case '\r':
			p[w] = '\n'
			w++
			// Swallow a paired \n so \r\n collapses to one \n.
			if r+1 < n && p[r+1] == '\n' {
				r++
			}
		default:
			if w != r {
				p[w] = p[r]
			}
			w++
		}
	}
	return w, err
}

// Write converts \n to \r\n and writes the result to the wrapped connection.
// A single per-connection buffer is reused across calls so the common case is
// zero-allocation after the first sufficiently-large write.
func (c *crlfWriter) Write(p []byte) (int, error) {
	need := len(p)
	for _, b := range p {
		if b == '\n' {
			need++
		}
	}
	if cap(c.buf) < need {
		c.buf = make([]byte, 0, need)
	}
	c.buf = c.buf[:0]
	for _, b := range p {
		if b == '\n' {
			c.buf = append(c.buf, '\r', '\n')
		} else {
			c.buf = append(c.buf, b)
		}
	}
	_, err := c.rw.Write(c.buf)
	// Return the original length so callers aren't confused by the size change.
	return len(p), err
}
