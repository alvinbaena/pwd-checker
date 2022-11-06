package gcs

import (
	"bufio"
	"fmt"
	"io"
)

// bitReader adds bit-level reading to os.File specifically.
type bitReader struct {
	inner  io.ReadSeeker
	buffer []byte
	unused uint8
}

func newBitReader(r io.ReadSeeker) *bitReader {
	return &bitReader{inner: r, buffer: make([]byte, 1), unused: 0}
}

// Reset the internal state of the bitReader. The next read will load fresh
// data from the current bitPos of the reader and start from the beginning
// of the first byte returned.
func (r *bitReader) Reset() {
	r.buffer[0] = 0
	r.unused = 0
}

// ReadBits reads up to 64 bits from the reader.
func (r *bitReader) ReadBits(n uint8) (uint64, error) {
	// Read up to 64 bits to the buffer
	if n > 64 {
		return 0, fmt.Errorf("cannot read more than 64 bits at a time")
	}

	ret := uint64(0)
	rBits := n

	for rBits > r.unused {
		ret |= uint64(r.buffer[0]) << (rBits - r.unused)
		rBits -= r.unused

		re, err := r.inner.Read(r.buffer)
		if err != nil {
			return uint64(re), err
		}

		r.unused = 8
	}

	if rBits > 0 {
		ret |= uint64(r.buffer[0]) >> (r.unused - rBits)
		r.buffer[0] &= (1 << (r.unused - rBits)) - 1
		r.unused -= rBits
	}

	return ret, nil
}

// Seek to the given *bit* position in the file.  Currently, only
// io.SeekStart and io.SeekEnd with negative offsets are supported.
func (r *bitReader) Seek(offset int64, whence int) (ret int64, err error) {
	switch whence {
	case io.SeekStart:
		r.Reset()
		ret, err = r.inner.Seek(offset/8, whence)
		if err != nil {
			return ret, err
		}
		if _, err = r.ReadBits(uint8(offset % 8)); err != nil {
			return 0, err
		}
		return offset, nil
	case io.SeekEnd:
		r.Reset()
		if offset < 0 {
			bypos := offset / 8
			bipos := 8 - (offset % 8)
			if bipos > 0 {
				bypos -= 1
			}
			ipos, err := r.inner.Seek(bypos, whence)
			if err != nil {
				return 0, err
			}

			if _, err = r.ReadBits(uint8(bipos)); err != nil {
				return 0, err
			}

			return ipos + (offset % 8), nil
		} else {
			return 0, fmt.Errorf("seeking past end of file not yet supported")
		}
	default:
		return 0, fmt.Errorf("current not yet supported")
	}
}

// An io.Writer and io.ByteWriter at the same time.
type writerAndByteWriter interface {
	io.Writer
	io.ByteWriter
}

// bitWriter adds bit-level writing to any io.Writer.
type bitWriter struct {
	inner   writerAndByteWriter
	wrapper *bufio.Writer // wrapper bufio.Writer if the target does not implement io.ByteWriter
	buffer  uint8         // unwritten bits are stored here
	unused  uint8         // number of unwritten bits in cache
}

func newBitWriter(out io.Writer) *bitWriter {
	w := &bitWriter{}
	var ok bool
	w.inner, ok = out.(writerAndByteWriter)
	if !ok {
		w.wrapper = bufio.NewWriter(out)
		w.inner = w.wrapper
	}
	return w
}

// WriteBits Writes up to 64 bits to the inner.
func (w *bitWriter) WriteBits(n uint8, r uint64) error {
	// Write up to 64 bits to the buffer
	if n > 64 {
		return fmt.Errorf("cannot write more than 64 bits at a time")
	}

	return w.writeBitsInternal(n, r&(1<<n-1))
}

// writeBitsInternal writes inner the 'n' lowest bits of r.
//
// r must not have bits set at n or higher positions (zero indexed).
// If r might not satisfy this, a mask must be explicitly applied
// before passing it to writeBitsInternal(), or WriteBits() should be used instead.
//
// writeBitsInternal() offers slightly better performance than WriteBits() because
// the input r is not masked. Calling writeBitsInternal() with an r that does
// not satisfy this is undefined behavior (might corrupt previously written bits).
//
// E.g. if you want to write 8 bits:
//
//	err := w.writeBitsInternal(0x34, 8)        // This is OK,
//	                                           // 0x34 has no bits set higher than the 8th
//	err := w.writeBitsInternal(0x1234&0xff, 8) // &0xff masks inner bits higher than the 8th
//
// Or:
//
//	err := w.WriteBits(0x1234, 8)            // bits higher than the 8th are ignored here
func (w *bitWriter) writeBitsInternal(n uint8, r uint64) error {
	newBits := w.unused + n
	if newBits < 8 {
		// r fits into buffer, no write will occur to file
		w.buffer |= byte(r) << (8 - newBits)
		w.unused = newBits
		return nil
	}
	if newBits > 8 {
		// buffer will be filled, and there will be more bits to write
		// "Fill buffer" and write it into inner
		free := 8 - w.unused
		if err := w.inner.WriteByte(w.buffer | uint8(r>>(n-free))); err != nil {
			return err
		}
		n -= free

		// write inner whole bytes
		for n >= 8 {
			n -= 8
			// No need to mask r, converting to byte will mask inner higher bits
			if err := w.inner.WriteByte(uint8(r >> n)); err != nil {
				return err
			}
		}
		// Put remaining into buffer
		if n > 0 {
			// Note: n < 8 (in case of n=8, 1<<n would overflow byte)
			w.buffer, w.unused = (uint8(r)&((1<<n)-1))<<(8-n), n
		} else {
			w.buffer, w.unused = 0, 0
		}
		return nil
	}

	// buffer will be filled exactly with the bits to be written
	bb := w.buffer | uint8(r)
	w.buffer, w.unused = 0, 0
	err := w.inner.WriteByte(bb)
	return err
}

// FlushBits aligns the bit stream to a byte boundary,
// so next write will start/go into a new byte.
// If there are cached bits, they are first written to the output.
// Returns the number of skipped (unset but still written) bits.
func (w *bitWriter) FlushBits() (skipped uint64, err error) {
	if w.unused > 0 {
		if err := w.inner.WriteByte(w.buffer); err != nil {
			return 0, err
		}

		skipped = uint64(8 - w.unused)
		w.buffer, w.unused = 0, 0
	}
	if w.wrapper != nil {
		err = w.wrapper.Flush()
	}
	return
}

// Flush any pending writes to the underlying buffer, padding with zero bits
// up to the nearest byte if necessary, and returning the number of padding
// bits written.  The sum of `WriteBits()` + `Flush()` or `FlushBits()`
// will be the total number of bits delivered to the inner, and will
// always end on a byte boundary.
//
// This also flushes the underlying inner.
func (w *bitWriter) Flush() (uint64, error) {
	wr, err := w.FlushBits()
	if err != nil {
		return wr, err
	}
	return wr, nil
}
