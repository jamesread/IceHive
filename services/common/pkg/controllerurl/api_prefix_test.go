package controllerurl

import "testing"

func TestWithAPIPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"http://host", "http://host/api"},
		{"http://host/", "http://host/api"},
		{"http://host/api", "http://host/api"},
		{"http://host/api/", "http://host/api"},
	}
	for _, tc := range tests {
		if got := WithAPIPrefix(tc.in); got != tc.want {
			t.Errorf("WithAPIPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
