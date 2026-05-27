package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/i18n"
)

// parseNamespaceFile accepts either {"namespace":[...]} or a bare [...] array.
func parseNamespaceFile(raw []byte) ([]any, error) {
	var wrapped struct {
		Namespace []any `json:"namespace"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Namespace != nil {
		return wrapped.Namespace, nil
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

func normalizeUNSPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	return strings.Trim(path, "/")
}

func joinUNSPath(parent, child string) string {
	parent = normalizeUNSPath(parent)
	child = normalizeUNSPath(child)
	switch {
	case parent == "":
		return child
	case child == "":
		return parent
	default:
		return parent + "/" + child
	}
}

// normalizeCreateNodeType maps CLI-friendly type names to OpenAPI values.
// Returns node "type" and optional "topicType" for file nodes.
// errOut may be nil; deprecation warnings are written to it when non-nil.
func normalizeCreateNodeType(nodeType, topicType string, errOut io.Writer) (string, string, error) {
	nodeType = strings.TrimSpace(nodeType)
	topicType = strings.TrimSpace(topicType)
	switch strings.ToLower(nodeType) {
	case "folder", "directory", "dir", "path":
		return "folder", "", nil
	case "file", "object", "topic":
		return "file", topicType, nil
	case "thing":
		if errOut != nil {
			fmt.Fprintln(errOut, i18n.T("warning: --type thing is deprecated, use --type file instead", "警告: --type thing 已废弃，请改用 --type file"))
		}
		return "file", topicType, nil
	case "metric", "action", "state":
		if topicType == "" {
			topicType = strings.ToLower(nodeType)
		}
		return "file", topicType, nil
	default:
		if nodeType == "" {
			return "", "", fmt.Errorf(i18n.T("invalid --type", "无效的 --type"))
		}
		// Pass through for forward compatibility (e.g. TOPIC).
		return nodeType, topicType, nil
	}
}

func buildLeafNode(name, nodeType, topicType, displayName, description, alias, fieldsJSON string, errOut io.Writer) (map[string]any, error) {
	typeStr, topicTypeStr, err := normalizeCreateNodeType(nodeType, topicType, errOut)
	if err != nil {
		return nil, err
	}
	node := map[string]any{
		"name": name,
		"type": typeStr,
	}
	if displayName != "" {
		node["displayName"] = displayName
	}
	if description != "" {
		node["description"] = description
	}
	if alias != "" {
		node["alias"] = alias
	}
	if topicTypeStr != "" {
		node["topicType"] = topicTypeStr
	}
	if fieldsJSON != "" {
		var fieldList []any
		if err := json.Unmarshal([]byte(fieldsJSON), &fieldList); err != nil {
			return nil, fmt.Errorf(i18n.T("invalid fields JSON: %w", "fields JSON 无效: %w"), err)
		}
		node["fields"] = fieldList
	}
	return node, nil
}

// buildNamespaceTreeFromPath turns "Plant/Line1/Metric/Temp" into a nested folder tree
// with the leaf node carrying metadata from leaf.
func buildNamespaceTreeFromPath(fullPath string, leaf map[string]any) ([]any, error) {
	fullPath = normalizeUNSPath(fullPath)
	if fullPath == "" {
		return nil, fmt.Errorf(i18n.T("topic path is empty", "topic 路径为空"))
	}
	segments := strings.Split(fullPath, "/")
	for _, seg := range segments {
		if seg == "" {
			return nil, fmt.Errorf(i18n.T("invalid topic path segment: %q", "无效的 topic 路径段: %q"), seg)
		}
	}
	leafName, _ := leaf["name"].(string)
	if leafName == "" {
		leafName = segments[len(segments)-1]
	}
	if leafName != segments[len(segments)-1] {
		return nil, fmt.Errorf(i18n.T(
			"leaf name %q does not match last path segment %q",
			"叶子节点名 %q 与路径最后一段 %q 不一致",
		), leafName, segments[len(segments)-1])
	}
	leaf["name"] = segments[len(segments)-1]

	if len(segments) == 1 {
		return []any{leaf}, nil
	}

	node := any(leaf)
	for i := len(segments) - 2; i >= 0; i-- {
		node = map[string]any{
			"name":     segments[i],
			"type":     "folder",
			"children": []any{node},
		}
	}
	return []any{node}, nil
}

func buildNamespaceFromFlags(parent, topic, nodeType, topicType, displayName, description, alias, fields string, errOut io.Writer) ([]any, string, error) {
	fullPath := joinUNSPath(parent, topic)
	fullPath = normalizeUNSPath(fullPath)
	if fullPath == "" {
		return nil, "", fmt.Errorf(i18n.T("topic path is empty", "topic 路径为空"))
	}
	segments := strings.Split(fullPath, "/")
	leafName := segments[len(segments)-1]
	leaf, err := buildLeafNode(leafName, nodeType, topicType, displayName, description, alias, fields, errOut)
	if err != nil {
		return nil, "", err
	}
	// Enforce: for file nodes the second-to-last segment must be the type folder (metric/action/state).
	if tt, _ := leaf["topicType"].(string); tt != "" {
		if len(segments) < 2 {
			return nil, "", fmt.Errorf(i18n.T(
				"path %q must include a type folder as second-to-last segment (e.g. .../metric/<name>) for --type %s",
				"路径 %q 的倒数第二段必须是类型文件夹（如 .../metric/<name>），当前 --type 为 %s",
			), fullPath, nodeType)
		}
		if got := strings.ToLower(segments[len(segments)-2]); got != tt {
			return nil, "", fmt.Errorf(i18n.T(
				"path %q: second-to-last segment must be %q for --type %s, got %q",
				"路径 %q：倒数第二段应为 %q（--type %s），实际为 %q",
			), fullPath, tt, nodeType, segments[len(segments)-2])
		}
	}
	namespace, err := buildNamespaceTreeFromPath(fullPath, leaf)
	if err != nil {
		return nil, "", err
	}
	return namespace, fullPath, nil
}
