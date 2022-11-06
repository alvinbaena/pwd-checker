package gcs

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestBuilder(t *testing.T) {
	file, err := os.Open("../../test/data/pwned-sample-sha1.txt")
	if err != nil {
		t.Fatalf("Should not fail opening file: %s", err)
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			t.Log("error closing Pwned Passwords file")
		}
	}(file)

	var writer bytes.Buffer
	builder := NewBuilder(file, &writer, 100, 16)
	err = builder.Process(true)
	if err != nil {
		t.Errorf("Should not fail processing file: %s", err)
	}

	if writer.Len() == 0 {
		t.Errorf("There should be data written")
	}

	t.Logf("Data len in Bytes: %d", writer.Len())

	reader := bytes.NewReader(writer.Bytes())
	_, err = reader.Seek(-8, io.SeekEnd)
	if err != nil {
		t.Errorf("Should not fail seeking offset from written: %s", err)
	}

	// Reads the footer that the file should have. 40 bytes.
	if _, err = reader.Seek(-40, io.SeekEnd); err != nil {
		t.Errorf("Should not fail seeking: %s", err)
	}

	buf := make([]byte, 8)
	if _, err = reader.Read(buf); err != nil {
		t.Errorf("Should not fail reading: %s", err)
	}
	if num := binary.BigEndian.Uint64(buf); num != 103 {
		t.Errorf("GCS should have %d items, have %d", 101, num)
	}

	buf = make([]byte, 8)
	if _, err = reader.Read(buf); err != nil {
		t.Errorf("Should not fail reading: %s", err)
	}
	if probability := binary.BigEndian.Uint64(buf); probability != 100 {
		t.Errorf("GCS should have probability %d, have %d", 100, probability)
	}

	buf = make([]byte, 8)
	if _, err = reader.Read(buf); err != nil {
		t.Errorf("Should not fail reading: %s", err)
	}
	if endOfData := binary.BigEndian.Uint64(buf); endOfData != 110 {
		t.Errorf("GCS should have end of data %d, have %d", 109, endOfData)
	}

	buf = make([]byte, 8)
	if _, err = reader.Read(buf); err != nil {
		t.Errorf("Should not fail reading: %s", err)
	}
	if indexLen := binary.BigEndian.Uint64(buf); indexLen != 7 {
		t.Errorf("GCS should have index length %d, have %d", 7, indexLen)
	}

	buf = make([]byte, 8)
	if _, err = reader.Read(buf); err != nil {
		t.Errorf("Should not fail reading: %s", err)
	}

	if string(buf) != gcsMagic {
		t.Errorf("Should not fail GCS footer")
	}
}
