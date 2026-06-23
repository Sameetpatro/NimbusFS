package client

import (
	"fmt"
	"io"
	"sync/atomic"
)

// ProgressWriter counts bytes written for simple cli progress display.
type ProgressWriter struct {
	total   int64
	written atomic.Int64
	label   string
}

// NewProgressWriter creates a progress counter optionally capped by total bytes.
func NewProgressWriter(label string, total int64) *ProgressWriter {
	return &ProgressWriter{label: label, total: total}
}

// Write implements io.Writer and prints progress as bytes flow through.
func (p *ProgressWriter) Write(b []byte) (int, error) {
	n := len(b)
	newTotal := p.written.Add(int64(n))
	if p.total > 0 {
		pct := float64(newTotal) / float64(p.total) * 100
		fmt.Printf("\r%s %d/%d bytes (%.1f%%)", p.label, newTotal, p.total, pct)
	} else {
		fmt.Printf("\r%s %d bytes", p.label, newTotal)
	}
	return n, nil
}

// Done prints a newline after transfer completes.
func (p *ProgressWriter) Done() {
	fmt.Println()
}

// Tee returns an io.Writer suitable for io.TeeReader.
func (p *ProgressWriter) Tee() io.Writer {
	return p
}
