package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── resolveNodeType ───────────────────────────────────────────────────────────

func TestResolveNodeType(t *testing.T) {
	tests := []struct {
		nt      string
		wantAPI string
		wantErr bool
	}{
		// canonical values
		{"topic", "TOPIC", false},
		{"path", "PATH", false},
		// legacy aliases — still accepted
		{"file", "TOPIC", false},
		{"object", "TOPIC", false},
		{"METRIC", "TOPIC", false},
		{"ACTION", "TOPIC", false},
		{"STATE", "TOPIC", false},
		{"thing", "TOPIC", false},
		{"FOLDER", "PATH", false},
		{"folder", "PATH", false},
		{"dir", "PATH", false},
		// errors
		{"", "", true},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		got, err := resolveNodeType(tt.nt, nil)
		if (err != nil) != tt.wantErr {
			t.Errorf("resolveNodeType(%q): err=%v wantErr=%v", tt.nt, err, tt.wantErr)
			continue
		}
		if err == nil && got != tt.wantAPI {
			t.Errorf("resolveNodeType(%q): got %q, want %q", tt.nt, got, tt.wantAPI)
		}
	}
}

// ── deriveTopicType ───────────────────────────────────────────────────────────

func TestDeriveTopicType_Valid(t *testing.T) {
	cases := []struct{ path, want string }{
		{"Plant/Line1/Metric/Temperature", "METRIC"},
		{"Factory1/Assembly/Line1/Station1/Metric/ProductionCount", "METRIC"},
		{"X/Action/Start", "ACTION"},
		{"Y/State/MachineStatus", "STATE"},
		// case-insensitive match
		{"Plant/metric/Temperature", "METRIC"},
		{"Plant/ACTION/StartCmd", "ACTION"},
	}
	for _, c := range cases {
		got, err := deriveTopicType(c.path)
		if err != nil {
			t.Errorf("deriveTopicType(%q): unexpected error: %v", c.path, err)
			continue
		}
		if got != c.want {
			t.Errorf("deriveTopicType(%q): got %q, want %q", c.path, got, c.want)
		}
	}
}

func TestDeriveTopicType_Invalid(t *testing.T) {
	cases := []struct {
		path        string
		errContains string
	}{
		// single segment — no parent
		{"Temperature", "type folder"},
		// parent is not a type folder
		{"Station1/ProductionCount", "Metric/Action/State"},
		{"Factory1/Assembly/Line1/Station1/ProductionCount", "Metric/Action/State"},
	}
	for _, c := range cases {
		_, err := deriveTopicType(c.path)
		if err == nil {
			t.Errorf("deriveTopicType(%q): expected error, got nil", c.path)
			continue
		}
		if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.errContains)) {
			t.Errorf("deriveTopicType(%q): error %q should contain %q", c.path, err, c.errContains)
		}
	}
}

// ── wrapInFolderTree ──────────────────────────────────────────────────────────

func TestWrapInFolderTree_SingleSegment(t *testing.T) {
	leaf := map[string]any{"name": "Plant", "type": "PATH"}
	ns, err := wrapInFolderTree("Plant", leaf)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 {
		t.Fatalf("len=%d", len(ns))
	}
	n := ns[0].(map[string]any)
	if n["name"] != "Plant" {
		t.Fatalf("name=%v", n["name"])
	}
}

func TestWrapInFolderTree_MultiLevel(t *testing.T) {
	leaf := map[string]any{"name": "Temperature", "type": "TOPIC", "topicType": "METRIC"}
	ns, err := wrapInFolderTree("Plant/Line1/Metric/Temperature", leaf)
	if err != nil {
		t.Fatal(err)
	}
	root := ns[0].(map[string]any)
	if root["name"] != "Plant" || root["type"] != "PATH" {
		t.Fatalf("root=%v", root)
	}
	// Walk Plant → Line1 → Metric → Temperature
	node := root
	for _, seg := range []string{"Line1", "Metric"} {
		ch := node["children"].([]any)[0].(map[string]any)
		if ch["name"] != seg {
			t.Fatalf("want segment %q, got %q", seg, ch["name"])
		}
		node = ch
	}
	l := node["children"].([]any)[0].(map[string]any)
	if l["name"] != "Temperature" || l["type"] != "TOPIC" {
		t.Fatalf("leaf=%v", l)
	}
}

// ── buildNamespaceFromFlags ───────────────────────────────────────────────────

func TestBuildNamespaceFromFlags_MetricPath(t *testing.T) {
	ns, path, err := buildNamespaceFromFlags(
		"", "Plant/Line1/Metric/Temperature",
		"topic", "", "Temp sensor", "", "",
		`[{"name":"value","type":"float","unit":"°C"}]`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "Plant/Line1/Metric/Temperature" {
		t.Fatalf("path=%q", path)
	}
	if len(ns) != 1 {
		t.Fatal("expected single root")
	}
}

func TestBuildNamespaceFromFlags_WithParent(t *testing.T) {
	_, fullPath, err := buildNamespaceFromFlags(
		"Factory1/Assembly/Line1/Station1", "Metric/ProductionCount",
		"topic", "", "Current output", "", "",
		`[{"name":"value","type":"int"}]`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fullPath != "Factory1/Assembly/Line1/Station1/Metric/ProductionCount" {
		t.Fatalf("fullPath=%q", fullPath)
	}
}

func TestBuildNamespaceFromFlags_ActionAndState(t *testing.T) {
	cases := []struct{ topic, nt, wantTT string }{
		{"Machine/Action/Start", "topic", "ACTION"},
		{"Machine/State/Status", "topic", "STATE"},
		{"Machine/Metric/Speed", "topic", "METRIC"},
	}
	for _, c := range cases {
		ns, _, err := buildNamespaceFromFlags("", c.topic, c.nt, "", "", "", "", "", nil)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", c.topic, err)
			continue
		}
		// Walk to leaf and check topicType
		node := ns[0].(map[string]any)
		for {
			ch, ok := node["children"]
			if !ok {
				break
			}
			node = ch.([]any)[0].(map[string]any)
		}
		if node["topicType"] != c.wantTT {
			t.Errorf("%q: topicType=%v, want %q", c.topic, node["topicType"], c.wantTT)
		}
	}
}

func TestBuildNamespaceFromFlags_FolderOnly(t *testing.T) {
	_, path, err := buildNamespaceFromFlags(
		"", "Plant/Line1", "path", "", "Line 1", "", "", "", nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "Plant/Line1" {
		t.Fatalf("path=%q", path)
	}
}

func TestBuildNamespaceFromFlags_StructuralErrors(t *testing.T) {
	bad := []struct {
		parent, topic, nt string
		desc              string
	}{
		{"", "Factory1/Assembly/Line1/Station1/ProductionCount", "topic",
			"leaf parent is Station1, not a type folder"},
		{"", "ProductionCount", "topic",
			"single segment, no type folder"},
		{"", "", "topic",
			"empty topic"},
	}
	for _, c := range bad {
		_, _, err := buildNamespaceFromFlags(c.parent, c.topic, c.nt, "", "", "", "", "", nil)
		if err == nil {
			t.Errorf("expected error (%s), got nil", c.desc)
		}
	}
}

// ── parseNamespaceFile ────────────────────────────────────────────────────────

func TestParseNamespaceFile_PathTopicPassThrough(t *testing.T) {
	// path/topic are passed through as-is; the backend accepts both natively.
	raw := []byte(`{"namespace":[
		{"name":"Line1","type":"path","children":[
			{"name":"Metric","type":"path","children":[
				{"name":"Count","type":"topic","topicType":"metric"}
			]}
		]}
	]}`)
	ns, err := parseNamespaceFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	root := ns[0].(map[string]any)
	if root["type"] != "path" {
		t.Errorf("path should be preserved: got %q", root["type"])
	}
	metric := root["children"].([]any)[0].(map[string]any)
	if metric["type"] != "path" {
		t.Errorf("nested path should be preserved: got %q", metric["type"])
	}
	leaf := metric["children"].([]any)[0].(map[string]any)
	if leaf["type"] != "topic" {
		t.Errorf("topic should be preserved: got %q", leaf["type"])
	}
}

func TestParseNamespaceFile_Wrapped(t *testing.T) {
	// "folder" is the legacy API name; backend accepts it, so it must pass through.
	ns, err := parseNamespaceFile([]byte(`{"namespace":[{"name":"a","type":"folder"}]}`))
	if err != nil || len(ns) != 1 {
		t.Fatalf("err=%v len=%d", err, len(ns))
	}
}

func TestParseNamespaceFile_BareArray(t *testing.T) {
	// "folder" legacy value must pass through unchanged.
	ns, err := parseNamespaceFile([]byte(`[{"name":"a","type":"folder"}]`))
	if err != nil || len(ns) != 1 {
		t.Fatalf("err=%v len=%d", err, len(ns))
	}
}

func TestParseNamespaceFile_FromDisk(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ns.json")
	raw, _ := json.Marshal(map[string]any{
		"namespace": []any{map[string]any{"name": "x", "type": "folder"}},
	})
	if err := os.WriteFile(f, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(f)
	ns, err := parseNamespaceFile(data)
	if err != nil || len(ns) != 1 {
		t.Fatalf("err=%v len=%d", err, len(ns))
	}
}
