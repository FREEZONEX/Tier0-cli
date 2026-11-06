package config

import "testing"

func TestSameOrigin(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		same bool
	}{
		{"https://example.test", "https://EXAMPLE.test:443/path", true},
		{"http://example.test", "http://example.test:80", true},
		{"https://example.test", "http://example.test", false},
		{"https://example.test", "https://example.test:444", false},
		{"https://example.test", "https://other.test", false},
		{"", "https://example.test", false},
	} {
		if SameOrigin(tc.a, tc.b) != tc.same {
			t.Errorf("SameOrigin(%q, %q)", tc.a, tc.b)
		}
	}
}

func TestSaveProfileClearsKeyWhenChangingOriginWithoutReplacement(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveProfile(Profile{BaseURL: "https://old.test", APIKey: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile(Profile{BaseURL: "https://new.test"}); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile()
	if err != nil || p.APIKey != "" {
		t.Fatal("old key persisted across origin change")
	}
}
