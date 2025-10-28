// Copyright 2025 Rui Yang (EPFL).
package mem

import ()

// KomaBuffer implements Buffer over an existing []byte without copying.
// It is NOT safe to modify the backing bytes and NOT safe for concurrent use.
type KomaBuffer struct {
	Data []byte
}

var _ Buffer = (*KomaBuffer)(nil)

// NewKomaBuffer wraps an existing slice as a Buffer. The caller must ensure
// the slice's lifetime exceeds that of all users of the returned Buffer.
func NewKomaBuffer(b []byte) Buffer {
	return &KomaBuffer{Data: b}
}

// ReadOnlyData returns the underlying byte slice (read-only view).
func (kb *KomaBuffer) ReadOnlyData() []byte { return kb.Data }

// Ref increments the reference count.
func (kb *KomaBuffer) Ref() {}

// Free decrements the reference count. When it reaches zero, nothing happens;
// Koma manages reclamation of the backing memory.
func (kb *KomaBuffer) Free() {}

// Len returns the number of bytes remaining.
func (kb *KomaBuffer) Len() int { return len(kb.Data) }

// split returns two Buffers that alias the same underlying bytes.
func (kb *KomaBuffer) split(n int) (left, right Buffer) {
	if n < 0 {
		n = 0
	}
	if n > len(kb.Data) {
		n = len(kb.Data)
	}
	left = &KomaBuffer{Data: kb.Data[:n:n]}
	right = &KomaBuffer{Data: kb.Data[n:len(kb.Data):len(kb.Data)]}
	return left, right
}

// read copies up to len(dst) bytes into dst and returns the remainder
// of this buffer as a new KomaBuffer (no extra copy beyond that memcpy).
func (kb *KomaBuffer) read(dst []byte) (int, Buffer) {
	if len(kb.Data) == 0 {
		return 0, kb
	}
	n := copy(dst, kb.Data)
	kb.Data = kb.Data[n:]
	return n, kb
}
