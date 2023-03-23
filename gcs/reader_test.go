// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package gcs

import (
	"crypto/sha1"
	"encoding/binary"
	"testing"
)

func TestReader_InvalidFile(t *testing.T) {
	reader := NewReader("../../test/data/pwned-sample-sha1.txt")
	if err := reader.Initialize(); err == nil {
		t.Errorf("Should fail")
	}
}

func TestReader(t *testing.T) {
	reader := NewReader("../../test/data/pwned-sample.gcs")
	if err := reader.Initialize(); err != nil {
		t.Errorf("Should not fail: %s", err)
	}

	h := sha1.New()
	h.Write([]byte("password"))
	buf := h.Sum(nil)
	hash := binary.BigEndian.Uint64(buf)

	exists, err := reader.Exists(hash)
	if err != nil {
		t.Errorf("Should not fail: %s", err)
	}

	if !exists {
		t.Errorf("Password should be on file")
	}

	h = sha1.New()
	h.Write([]byte("1mag@saG(@31*sasd."))
	buf = h.Sum(nil)
	hash = binary.BigEndian.Uint64(buf)

	exists, err = reader.Exists(hash)
	if err != nil {
		t.Errorf("Should not fail: %s", err)
	}

	if exists {
		t.Errorf("Password should not be on file")
	}
}
