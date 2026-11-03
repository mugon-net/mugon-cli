package command

import "testing"

func TestFrameAncestors(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"localhost", "http://localhost:3000/ http://*.localhost:3000/"},
		{"sim-ab12-1.localhost", "http://sim-ab12-1.localhost:3000/ http://localhost:3000/ http://*.localhost:3000/"},
		{"192.168.1.5", "http://192.168.1.5:3000/"},
		{"[::1]", "http://[::1]:3000/"},
	}
	for _, test := range tests {
		if got := frameAncestors(test.host, 3000); got != test.want {
			t.Errorf("frameAncestors(%q) = %q, want %q", test.host, got, test.want)
		}
	}
}
