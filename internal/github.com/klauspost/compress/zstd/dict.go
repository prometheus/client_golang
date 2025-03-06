package zstd

import (
	"github.com/prometheus/client_golang/internal/github.com/klauspost/compress/huff0"
)

type dict struct {
	id uint32

	litEnc  *huff0.Scratch
	offsets [3]int
	content []byte
}

const dictMagic = "\x37\xa4\x30\xec"

// Maximum dictionary size for the reference implementation (1.5.3) is 2 GiB.
const dictMaxLength = 1 << 31

// ID returns the dictionary id or 0 if d is nil.
func (d *dict) ID() uint32 {
	if d == nil {
		return 0
	}
	return d.id
}

// ContentSize returns the dictionary content size or 0 if d is nil.
func (d *dict) ContentSize() int {
	if d == nil {
		return 0
	}
	return len(d.content)
}

// Content returns the dictionary content.
func (d *dict) Content() []byte {
	if d == nil {
		return nil
	}
	return d.content
}

// Offsets returns the initial offsets.
func (d *dict) Offsets() [3]int {
	if d == nil {
		return [3]int{}
	}
	return d.offsets
}

// LitEncoder returns the literal encoder.
func (d *dict) LitEncoder() *huff0.Scratch {
	if d == nil {
		return nil
	}
	return d.litEnc
}
