package cmd

import "testing"

// TestNormalizePort pins the `serve -p` shorthand: a bare port is expanded to
// HOST:CONTAINER with the same number on both sides, while any value that already
// contains a colon (an explicit HOST:CONTAINER, or an IP:HOST:CONTAINER) passes through
// unchanged.
func TestNormalizePort(t *testing.T) {
	cases := map[string]string{
		"3000":              "3000:3000",         // bare port mirrored to both sides
		"8080:80":           "8080:80",           // explicit HOST:CONTAINER untouched
		"127.0.0.1:8080:80": "127.0.0.1:8080:80", // IP-qualified mapping untouched
	}
	for in, want := range cases {
		if got := normalizePort(in); got != want {
			t.Errorf("normalizePort(%q) = %q, want %q", in, got, want)
		}
	}
}
