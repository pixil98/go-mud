package listener

import (
	"bytes"
	"io"
)

// crlfWriter wraps an io.ReadWriter and converts \n to \r\n on writes.
// This is needed for protocols like telnet that require CRLF line endings.
type crlfWriter struct {
	rw io.ReadWriter
}

func newCRLFReadWriter(rw io.ReadWriter) io.ReadWriter {
	return &crlfWriter{rw: rw}
}

func (c *crlfWriter) Read(p []byte) (int, error) {
	return c.rw.Read(p)
}

func (c *crlfWriter) Write(p []byte) (int, error) {
	converted := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
	_, err := c.rw.Write(converted)
	// Return the original length so callers aren't confused by the size change
	return len(p), err
}
