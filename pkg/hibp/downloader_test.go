// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package hibp

import (
	"os"
	"testing"
)

func TestDownloader(t *testing.T) {
	file, err := os.Create("../../test/data/download-test-1.txt")
	if err != nil {
		t.Errorf("Should not fail creating a file: %s", err)
	}

	downloader := NewDownloader(file, 1)
	if err = downloader.ProcessRanges(1, true); err != nil {
		t.Errorf("Should not fail download: %s", err)
	}

	stat, err := os.Stat(file.Name())
	if err != nil {
		t.Errorf("Should not fail stat'ing the file: %s", err)
	}

	if stat.Size() == 0 {
		t.Errorf("File should have a positive size")
	}

	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Fatalf("Should not fail closing file: %s", err)
		}

		if err := os.Remove(file.Name()); err != nil {
			t.Fatalf("Should not fail deleting file: %s", err)
		}
	})
}

func TestDownloader_Parallel(t *testing.T) {
	file, err := os.Create("../../test/data/download-test-2.txt")
	if err != nil {
		t.Errorf("Should not fail creating a file: %s", err)
	}

	downloader := NewDownloader(file, 0)
	if err = downloader.ProcessRanges(1, true); err != nil {
		t.Errorf("Should not fail download: %s", err)
	}

	stat, err := os.Stat(file.Name())
	if err != nil {
		t.Errorf("Should not fail stat'ing the file: %s", err)
	}

	if stat.Size() == 0 {
		t.Errorf("File should have a positive size")
	}

	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Fatalf("Should not fail closing file: %s", err)
		}

		if err := os.Remove(file.Name()); err != nil {
			t.Fatalf("Should not fail deleting file: %s", err)
		}
	})
}
