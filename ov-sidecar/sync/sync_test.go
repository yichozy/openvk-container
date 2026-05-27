package sync

import (
	"testing"
)

func TestValidateDestURL(t *testing.T) {
	tests := []struct {
		dest string
		ok   bool
	}{
		{"rsync://replica:1935/viking/", true},
		{"rsync://10.0.0.1:873/data/", true},
		{"http://example.com/data", false},
		{"/local/path", false},
		{"rsync://", false},
		{"rsync:///viking/", false},
		{"rsync://host", false},
		{"", false},
	}
	for _, tc := range tests {
		err := validateDestURL(tc.dest)
		if tc.ok && err != nil {
			t.Errorf("expected ok for %q, got %v", tc.dest, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("expected error for %q", tc.dest)
		}
	}
}
