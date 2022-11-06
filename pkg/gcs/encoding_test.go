package gcs

import (
	"bytes"
	"testing"
)

func TestGolombEncoder(t *testing.T) {
	cases := []struct {
		inputs      []uint64
		probability uint64
		want        []uint64
		fail        bool
	}{
		{[]uint64{42, 74, 96, 32}, 4, []uint64{13, 21, 27, 11}, false},
		{[]uint64{420}, 2, []uint64{}, true},
	}

	for _, tc := range cases {
		var buf bytes.Buffer
		encoder := newEncoder(&buf, tc.probability)

		for i, val := range tc.inputs {
			wr, err := encoder.Encode(val)
			if !tc.fail {
				if err != nil {
					t.Errorf("Encode should not fail: %s", err)
				}
				if tc.want[i] != wr {
					t.Errorf("Encode(%d): %d, want: %d", val, wr, tc.want[i])
				}
			} else {
				if err == nil {
					t.Errorf("Encode should not fail: %s", err)
				}
			}
		}

		if !tc.fail {
			_, err := encoder.Finalize()
			if err != nil {
				t.Errorf("Finalize should not fail: %s", err)
			}
		}
	}
}
