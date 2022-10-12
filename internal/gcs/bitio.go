package gcs

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// https://github.com/Freaky/rust-bitrw

// BitReader adds bit-level reading to os.File specifically.
// TODO Maybe implement using bufio?
type BitReader struct {
	inner  *os.File
	buffer []byte
	unused uint8
}

func NewBitReader(w *os.File) *BitReader {
	return &BitReader{inner: w, buffer: make([]byte, 1), unused: 0}
}

// Reset the internal state of the BitReader. The next read will load fresh
// data from the current bitPos of the reader and start from the beginning
// of the first byte returned.
func (r *BitReader) Reset() {
	r.buffer[0] = 0
	r.unused = 0
}

// ReadBit reads a single bit from the reader.
func (r *BitReader) ReadBit() (uint8, error) {
	bit, err := r.ReadBits(1)
	return uint8(bit), err
}

// ReadBits reads up to 64 bits from the reader.
func (r *BitReader) ReadBits(nBits uint8) (uint64, error) {
	// Read up to 64 bits to the buffer
	if nBits > 64 {
		return 0, errors.New("cannot read more than 64 bits at a time")
	}

	ret := uint64(0)
	rBits := nBits

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
		r.buffer[0] &= byte(mask(uint64(r.unused - rBits)))
		r.unused -= rBits
	}

	return ret, nil
}

// Seek to the given *bit* bitPos in the file.  Currently, only
// io.SeekStart and io.SeekEnd with negative offsets are supported.
func (r *BitReader) Seek(offset int64, whence int) (ret int64, err error) {
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
			ipos, err := r.inner.Seek(bypos, io.SeekEnd)
			if err != nil {
				return 0, err
			}

			if _, err = r.ReadBits(uint8(bipos)); err != nil {
				return 0, err
			}

			return ipos + (offset % 8), nil
		} else {
			return 0, errors.New("seeking past end of file not yet supported")
		}
	default:
		return 0, errors.New("current not yet supported")
	}
}

// IntoInner unwraps this BitReader, returning the underlying io.File and discarding any
// unread buffered bits.
func (r *BitReader) IntoInner() *os.File {
	return r.inner
}

// BitWriter adds bit-level writing to any io.Writer.
type BitWriter struct {
	inner  *os.File
	buffer uint64
	unused uint64
}

func NewBitWriter(w *os.File) *BitWriter {
	return &BitWriter{inner: w}
}

// WriteBit writes a single bit to the inner.
func (w *BitWriter) WriteBit(bit uint8) (uint64, error) {
	if bit > 0 {
		return 0, errors.New("bit has no content")
	}

	return w.WriteBits(1, uint64(bit))
}

// WriteBits Writes up to 64 bits to the inner.
func (w *BitWriter) WriteBits(nBits uint8, value uint64) (uint64, error) {
	// Write up to 64 bits to the buffer
	if nBits > 64 {
		return 0, errors.New("cannot write more than 64 bits at a time")
	}

	nBitsRemaining := uint64(nBits)
	if nBitsRemaining >= w.unused && w.unused < 8 {
		excessBits := nBitsRemaining - w.unused
		w.buffer <<= w.unused
		w.buffer |= (value >> excessBits) & mask(w.unused)

		buf := make([]byte, binary.MaxVarintLen64)
		wr := binary.PutUvarint(buf, w.buffer)
		if err := binary.Write(w.inner, binary.BigEndian, buf[:wr]); err != nil {
			return uint64(wr * 8), err
		}

		nBitsRemaining = excessBits
		w.unused = 8
		w.buffer = 0
	}

	// let's write while we can fill up full bytes
	for nBitsRemaining >= 8 {
		nBitsRemaining -= 8

		buf := make([]byte, binary.MaxVarintLen64)
		wr := binary.PutUvarint(buf, w.buffer)
		if err := binary.Write(w.inner, binary.BigEndian, buf[:wr]); err != nil {
			return uint64(wr * 8), err
		}
	}

	// put the remaining bits in the buffer
	if nBitsRemaining > 0 {
		w.buffer <<= nBitsRemaining
		w.buffer |= value & mask(nBitsRemaining)
		w.unused -= nBitsRemaining
	}

	return uint64(nBits), nil
}

// FlushBits is exactly the same as Flush(), only it doesn't call Flush() on the
// wrapped inner.
//
// This may be useful if you're going to call `into_inner()` to get at the
// wrapped inner in order to perform more bytewise writes, and don't care
// if it's all on stable storage just yet.
func (w *BitWriter) FlushBits() (uint64, error) {
	if w.unused != 8 {
		buf := make([]byte, binary.MaxVarintLen64)
		wr := binary.PutUvarint(buf, w.buffer)
		if err := binary.Write(w.inner, binary.BigEndian, buf[:wr]); err != nil {
			return 0, err
		}

		written := w.unused
		w.unused = 8
		return written, nil
	}

	return 0, nil
}

// Flush any pending writes to the underlying buffer, padding with zero bits
// up to the nearest byte if necessary, and returning the number of padding
// bits written.  The sum of `WriteBits()` + `Flush()` or `FlushBits()`
// will be the total number of bits delivered to the inner, and will
// always end on a byte boundary.
//
// This method should **always** be called prior to calling IntoInner() or
// before allowing the `BitWriter` to go out of scope, or buffered bytes may
// be lost.
//
// This also flushes the underlying inner.
func (w *BitWriter) Flush() (uint64, error) {
	wr, err := w.FlushBits()
	if err != nil {
		return wr, err
	}

	//err = (*w.inner).Flush()
	//if err != nil {
	//	return wr, err
	//}

	return wr, nil
}

// IntoInner unwraps this BitWriter, returning the underlying inner and discarding any
// unwritten buffered bits.
//
// You should call Flush() if this is undesirable.
func (w *BitWriter) IntoInner() *os.File {
	return w.inner
}

func mask(n uint64) uint64 {
	return (1 << n) - 1
}
