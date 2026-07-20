package main

import "testing"

func TestJSONRequested(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "bare flag", args: []string{"--json", "unknown"}, want: true},
		{name: "explicit true", args: []string{"unknown", "--json=true"}, want: true},
		{name: "explicit false wins", args: []string{"--json", "--json=false"}, want: false},
		{name: "positional after separator", args: []string{"unknown", "--", "--json"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := jsonRequested(test.args); got != test.want {
				t.Fatalf("jsonRequested(%q) = %v, want %v", test.args, got, test.want)
			}
		})
	}
}
