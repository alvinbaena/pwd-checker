// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package gcs

import (
	"bytes"
	"io"
	"testing"
)

// testWriter that does not implement io.ByteWriter so we can test the
// behaviour of Writer when it creates an internal bufio.Writer.
type testWriter struct {
	b *bytes.Buffer
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	return w.b.Write(p)
}

func (w *testWriter) Bytes() []byte {
	return w.b.Bytes()
}

func TestBitWriter(t *testing.T) {
	cases := []struct {
		size   uint8
		inputs []uint64
		want   []byte
		fail   bool
	}{
		{8, []uint64{255}, []byte{0xff}, false},
		{4, []uint64{15, 15}, []byte{0xff}, false},
		{2, []uint64{3, 3, 3, 3}, []byte{0xff}, false},
		{1, []uint64{1, 1, 1, 1, 1, 1, 1, 1}, []byte{0xff}, false},
		{4, []uint64{15, 15, 15}, []byte{0xff, 0xf0}, false},
		{2, []uint64{3, 3, 3, 3, 3, 3}, []byte{0xff, 0xf0}, false},
		{65, []uint64{255}, []byte{0xff, 0xff, 0xff}, true},
		{64, []uint64{255}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff}, false},
		{16, []uint64{65535}, []byte{0xff, 0xff}, false},
		{14, []uint64{255, 128}, []byte{0x03, 0xfc, 0x08, 0x00}, false},
	}

	// first cases for writer with io.ByteWriter support
	for _, tc := range cases {
		var buf bytes.Buffer
		writer := newBitWriter(&buf)

		for _, input := range tc.inputs {
			err := writer.WriteBits(tc.size, input)
			if !tc.fail {
				if err != nil {
					t.Errorf("Write should not fail: %s", err)
				}
			} else {
				if err == nil {
					t.Errorf("Write should fail: %s", err)
				}
			}
		}

		if !tc.fail {
			if _, err := writer.Flush(); err != nil {
				t.Errorf("Flush should not fail: %s", err)
			}

			if !bytes.Equal(buf.Bytes(), tc.want) {
				t.Errorf("Write writes: %x, want: %x", buf.Bytes(), tc.want)
			}
		}
	}

	// second cases for writer without io.ByteWriter support
	for _, tc := range cases {
		var buf bytes.Buffer
		inner := &testWriter{b: &buf}
		writer := newBitWriter(inner)

		for _, input := range tc.inputs {
			err := writer.WriteBits(tc.size, input)
			if !tc.fail {
				if err != nil {
					t.Errorf("Write should not fail: %s", err)
				}
			} else {
				if err == nil {
					t.Errorf("Write should fail: %s", err)
				}
			}
		}

		if !tc.fail {
			if _, err := writer.Flush(); err != nil {
				t.Errorf("Flush should not fail: %s", err)
			}

			if !bytes.Equal(buf.Bytes(), tc.want) {
				t.Errorf("Write writes: %x, want: %x", buf.Bytes(), tc.want)
			}
		}
	}
}

func TestBitReader(t *testing.T) {
	cases := []struct {
		n      uint8
		inputs []byte
		want   uint64
		fail   bool
	}{
		{8, []byte{3}, 3, false},
		{1, []byte{0x1}, 0, false},
		{1, []byte{0xf0}, 1, false},
		{8, []byte{255}, 255, false},
		{65, []byte{255}, 255, true},
		{4, []byte{0xcc}, 0xc, false},
	}

	for _, tc := range cases {
		reader := newBitReader(bytes.NewReader(tc.inputs))

		got, err := reader.ReadBits(tc.n)
		if !tc.fail {
			if err != nil && err != io.EOF {
				t.Fatalf("ReadBits(%d) should not fail: %s", tc.n, err)
			}
			if err != nil {
				t.Errorf("ReadBits should not fail: %s", err)
			}
			if got != tc.want {
				t.Errorf("ReadBits(%d): %b, want: %b", tc.n, got, tc.want)
			}
		} else {
			if err == nil {
				t.Errorf("ReadBits should fail: %s", err)
			}
		}
	}
}

func TestBitReader_Seek(t *testing.T) {
	cases := []struct {
		offset int64
		whence int
		inputs []byte
		want   int64
		fail   bool
	}{
		{8, io.SeekStart, []byte{255}, 8, false},
		{12, io.SeekStart, []byte{0b1111}, 12, true},
		{8, io.SeekCurrent, []byte{}, 0, true},
		// Test only io.SeekEnd errors, as bytes.Reader does not implement it
		{8, io.SeekEnd, []byte{255}, 0, true},
		{-2, io.SeekEnd, []byte{255}, 0, true},
		{-8, io.SeekEnd, []byte{255, 255, 255}, 0b1, false},
		{-16, io.SeekEnd, []byte{255, 128, 255, 128}, 0b1, false},
	}

	for _, tc := range cases {
		reader := newBitReader(bytes.NewReader(tc.inputs))
		got, err := reader.Seek(tc.offset, tc.whence)
		if !tc.fail {
			if err != nil {
				t.Errorf("Seek should not fail: %s", err)
			}
			if got != tc.want {
				t.Errorf("Seek(%d, %d): %b, want: %b", tc.offset, tc.whence, got, tc.want)
			}
		} else {
			if err == nil {
				t.Errorf("Seek should fail: %s", err)
			}
		}
	}
}
