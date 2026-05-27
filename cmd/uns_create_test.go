package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseNamespaceFile_Wrapped(t *testing.T) {
	raw := []byte(`{"namespace":[{"name":"a","type":"folder"}]}`)
	ns, err := parseNamespaceFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 {
		t.Fatalf("len=%d want 1", len(ns))
	}
}

func TestParseNamespaceFile_Array(t *testing.T) {
	raw := []byte(`[{"name":"a","type":"folder"}]`)
	ns, err := parseNamespaceFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 {
		t.Fatalf("len=%d want 1", len(ns))
	}
}

func TestBuildNamespaceTreeFromPath_MultiLevel(t *testing.T) {
	leaf, err := buildLeafNode("Temperature", "METRIC", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	ns, err := buildNamespaceTreeFromPath("Plant/Line1/Metric/Temperature", leaf)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 {
		t.Fatalf("root count=%d want 1", len(ns))
	}
	root, ok := ns[0].(map[string]any)
	if !ok || root["name"] != "Plant" || root["type"] != "folder" {
		t.Fatalf("root=%v", ns[0])
	}
	children, ok := root["children"].([]any)
	if !ok || len(children) != 1 {
		t.Fatalf("children=%v", root["children"])
	}
	// Walk to leaf
	node := children[0].(map[string]any)
	for node["name"] != "Metric" {
		ch, ok := node["children"].([]any)
		if !ok || len(ch) != 1 {
			t.Fatalf("unexpected node at %v", node["name"])
		}
		node = ch[0].(map[string]any)
	}
	leafNode := node["children"].([]any)[0].(map[string]any)
	if leafNode["name"] != "Temperature" {
		t.Fatalf("leaf name=%v", leafNode["name"])
	}
	if leafNode["type"] != "file" || leafNode["topicType"] != "metric" {
		t.Fatalf("leaf=%v", leafNode)
	}
}

func TestBuildNamespaceFromFlags_WithParent(t *testing.T) {
	ns, fullPath, err := buildNamespaceFromFlags("Plant", "Line1/Metric/Temp", "METRIC", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if fullPath != "Plant/Line1/Metric/Temp" {
		t.Fatalf("fullPath=%q", fullPath)
	}
	if len(ns) != 1 {
		t.Fatal("expected single root")
	}
}

func TestNormalizeCreateNodeType(t *testing.T) {
	tests := []struct {
		inType, inTopicType, wantType, wantTopicType string
	}{
		{"FOLDER", "", "folder", ""},
		{"METRIC", "", "file", "metric"},
		{"thing", "", "file", ""},
		{"file", "action", "file", "action"},
	}
	for _, tt := range tests {
		gotType, gotTopicType, err := normalizeCreateNodeType(tt.inType, tt.inTopicType)
		if err != nil {
			t.Fatalf("%s: %v", tt.inType, err)
		}
		if gotType != tt.wantType || gotTopicType != tt.wantTopicType {
			t.Fatalf("%s: got %s/%s want %s/%s", tt.inType, gotType, gotTopicType, tt.wantType, tt.wantTopicType)
		}
	}
}

func TestParseNamespaceFile_FromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "structure.json")
	body := map[string]any{
		"namespace": []any{
			map[string]any{"name": "factory", "type": "folder"},
		},
	}
	raw, _ := json.Marshal(body)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ns, err := parseNamespaceFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 {
		t.Fatalf("len=%d", len(ns))
	}
}
