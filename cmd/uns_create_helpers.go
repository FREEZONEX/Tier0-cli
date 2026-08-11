package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ── path utilities ────────────────────────────────────────────────────────────

func normalizeUNSPath(p string) string {
	return strings.Trim(strings.ReplaceAll(p, "\\", "/"), "/")
}

func joinUNSPath(parent, child string) string {
	p, c := normalizeUNSPath(parent), normalizeUNSPath(child)
	switch {
	case p == "":
		return c
	case c == "":
		return p
	default:
		return p + "/" + c
	}
}

// ── file parsing ──────────────────────────────────────────────────────────────

// parseNamespaceFile accepts {"namespace":[...]} or a bare [...] array.
// The backend accepts canonical PATH/TOPIC and legacy path/topic/folder/file
// interchangeably, so user-provided JSON is forwarded as-is.
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

// validateNamespaceTree catches request-shape errors locally so --dry-run is a
// meaningful preflight rather than only a JSON syntax check. The create API
// expects every node to use "name" (not "path"), and Metric topics must
// declare at least one schema field.
func validateNamespaceTree(namespace []any) error {
	if len(namespace) == 0 {
		return fmt.Errorf("namespace must contain at least one node")
	}
	return validateNamespaceNodes(namespace, nil, "namespace")
}

func validateNamespaceNodes(nodes []any, parents []string, location string) error {
	for i, raw := range nodes {
		itemLocation := fmt.Sprintf("%s[%d]", location, i)
		node, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be a JSON object", itemLocation)
		}

		name, ok := node["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			if _, hasPath := node["path"]; hasPath {
				return fmt.Errorf("%s.name is required; use \"name\" for a node label, not \"path\"", itemLocation)
			}
			return fmt.Errorf("%s.name is required", itemLocation)
		}

		typeValue, ok := node["type"].(string)
		if !ok || strings.TrimSpace(typeValue) == "" {
			return fmt.Errorf("%s.type is required", itemLocation)
		}
		typeValue = strings.ToLower(strings.TrimSpace(typeValue))
		isTopic := typeValue == "topic" || typeValue == "file" || typeValue == "object" ||
			typeValue == "metric" || typeValue == "action" || typeValue == "state" || typeValue == "thing"
		isPath := typeValue == "path" || typeValue == "folder" || typeValue == "directory" || typeValue == "dir"
		if !isTopic && !isPath {
			return fmt.Errorf("%s.type %q is invalid; use PATH or TOPIC", itemLocation, node["type"])
		}

		fullPath := strings.Join(append(append([]string{}, parents...), name), "/")
		if isTopic {
			topicType, err := deriveTopicType(fullPath)
			if err != nil {
				return fmt.Errorf("%s: %w", itemLocation, err)
			}
			if topicType == "METRIC" {
				fields, ok := node["fields"].([]any)
				if !ok || len(fields) == 0 {
					return fmt.Errorf("%s.fields is required for Metric topic %q", itemLocation, fullPath)
				}
			}
		}

		if childrenRaw, exists := node["children"]; exists {
			children, ok := childrenRaw.([]any)
			if !ok {
				return fmt.Errorf("%s.children must be a JSON array", itemLocation)
			}
			if isTopic && len(children) > 0 {
				return fmt.Errorf("%s.children is not allowed on topic %q", itemLocation, fullPath)
			}
			if len(children) > 0 {
				childParents := append(append([]string{}, parents...), name)
				if err := validateNamespaceNodes(children, childParents, itemLocation+".children"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ── node type resolution ──────────────────────────────────────────────────────

// typeFolders is the set of valid UNS type-folder names (lower-cased key → display name).
// These must appear as the second-to-last path segment for every file (topic) node.
var typeFolders = map[string]string{
	"metric": "Metric",
	"action": "Action",
	"state":  "State",
}

// resolveNodeType maps the --type flag to the API node type.
//
//	--type path   → "PATH"  node  (a directory in the UNS tree)
//	--type topic  → "TOPIC" node  (a data point / topic)
//
// Legacy aliases are accepted with a deprecation warning so existing scripts
// keep working during migration.
func resolveNodeType(nodeTypeFlag string, errOut io.Writer) (apiType string, err error) {
	warn := func(old, new string) {
		if errOut != nil {
			fmt.Fprintln(errOut,
				"warning: --type "+old+" is deprecated, use --type "+new,
			)
		}
	}

	switch strings.ToLower(strings.TrimSpace(nodeTypeFlag)) {
	case "topic":
		return "TOPIC", nil
	case "path":
		return "PATH", nil

	// ── legacy aliases (kept for backward compatibility) ──────────────────
	case "file", "object":
		warn(nodeTypeFlag, "topic")
		return "TOPIC", nil
	case "metric", "action", "state":
		warn(nodeTypeFlag, "topic")
		return "TOPIC", nil
	case "thing":
		warn("thing", "topic")
		return "TOPIC", nil
	case "folder", "directory", "dir":
		warn(nodeTypeFlag, "path")
		return "PATH", nil

	case "":
		return "", fmt.Errorf(
			"--type is required: use 'path' for folders or 'topic' for data points",
		)
	default:
		return "", fmt.Errorf(
			"--type %q is not valid: use 'path' for folders or 'topic' for data points",
			nodeTypeFlag)
	}
}

// ── structural validation ─────────────────────────────────────────────────────

// deriveTopicType enforces the UNS structural rule and returns the topicType.
//
// Rule: for every file (topic) node the segment immediately before the leaf
// must be a type folder — one of Metric, Action, or State.
// The topicType is derived from that segment; it is never injected or guessed.
//
// Valid:
//
//	.../Metric/ProductionCount   → topicType "METRIC"
//	.../Action/StartCommand      → topicType "ACTION"
//	.../State/MachineStatus      → topicType "STATE"
//
// Invalid:
//
//	.../Station1/ProductionCount  (Station1 is not a type folder)
//	ProductionCount               (no parent segment at all)
func deriveTopicType(fullPath string) (topicType string, err error) {
	segments := strings.Split(normalizeUNSPath(fullPath), "/")
	if len(segments) < 2 {
		return "", fmt.Errorf(
			"path %q: a topic node needs at least two segments — "+
				"a type folder (Metric/Action/State) immediately before the leaf name.\n"+
				"  Example: .../Metric/%s",

			fullPath, segments[len(segments)-1])
	}

	parent := strings.ToLower(segments[len(segments)-2])
	if _, ok := typeFolders[parent]; !ok {
		suggested := strings.Join(segments[:len(segments)-1], "/") +
			"/Metric/" + segments[len(segments)-1]
		return "", fmt.Errorf(
			"path %q: segment before leaf must be a type folder (Metric/Action/State), got %q.\n"+
				"  Type folders are never inserted automatically — include one in your path.\n"+
				"  Example: %s",

			fullPath, segments[len(segments)-2], suggested)
	}

	return strings.ToUpper(parent), nil // e.g. "METRIC", "ACTION", "STATE"
}

// ── namespace tree construction ───────────────────────────────────────────────

// buildLeafNode constructs the JSON node map for the leaf (topic or folder).
func buildLeafNode(name, apiType, topicType, displayName, description, alias, fieldsJSON string) (map[string]any, error) {
	node := map[string]any{
		"name": name,
		"type": apiType,
	}
	if topicType != "" {
		node["topicType"] = topicType
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
	if fieldsJSON != "" {
		var fields []any
		if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
			return nil, fmt.Errorf("--fields JSON is invalid: %w", err)
		}
		node["fields"] = fields
	}
	return node, nil
}

// wrapInFolderTree wraps the leaf in a nested path chain for fullPath.
// Every segment except the last becomes a path node; the last segment is the leaf.
//
// Example: fullPath = "Plant/Line1/Metric/Temp", leaf = {name:"Temp", type:"TOPIC"} →
//
//	[{name:"Plant", type:"PATH", children:[
//	  {name:"Line1", type:"PATH", children:[
//	    {name:"Metric", type:"PATH", children:[
//	      {name:"Temp", type:"TOPIC", …}
//	    ]}
//	  ]}
//	]}]
func wrapInFolderTree(fullPath string, leaf map[string]any) ([]any, error) {
	path := normalizeUNSPath(fullPath)
	if path == "" {
		return nil, fmt.Errorf("topic path is empty")
	}
	segments := strings.Split(path, "/")
	for _, seg := range segments {
		if seg == "" {
			return nil, fmt.Errorf("topic path contains an empty segment")
		}
	}
	// Authoritative: leaf name is always the last path segment.
	leaf["name"] = segments[len(segments)-1]

	if len(segments) == 1 {
		return []any{leaf}, nil
	}

	var node any = leaf
	for i := len(segments) - 2; i >= 0; i-- {
		node = map[string]any{
			"name":     segments[i],
			"type":     "PATH",
			"children": []any{node},
		}
	}
	return []any{node}, nil
}

// ── public entry point ────────────────────────────────────────────────────────

// buildNamespaceFromFlags is the single entry point for the --topic mode.
// It validates all flags, resolves the node type, enforces the structural rule,
// and returns the namespace array ready to POST plus the resolved full path.
//
// For file (topic) nodes the topicType is derived from the path structure:
// the segment before the leaf must be Metric, Action, or State.
func buildNamespaceFromFlags(
	parent, topic,
	nodeTypeFlag, topicTypeFlag,
	displayName, description, alias, fieldsJSON string,
	errOut io.Writer,
) (namespace []any, fullPath string, err error) {
	// The topic-type flag is deprecated; topicType is derived from the path.
	_ = topicTypeFlag

	fullPath = normalizeUNSPath(joinUNSPath(parent, topic))
	if fullPath == "" {
		return nil, "", fmt.Errorf("--topic is required")
	}

	apiType, err := resolveNodeType(nodeTypeFlag, errOut)
	if err != nil {
		return nil, "", err
	}

	// For topic nodes: derive topicType from path (enforces structural rule).
	topicType := ""
	if apiType == "TOPIC" {
		topicType, err = deriveTopicType(fullPath)
		if err != nil {
			return nil, "", err
		}
	}

	segments := strings.Split(fullPath, "/")
	leaf, err := buildLeafNode(
		segments[len(segments)-1],
		apiType, topicType,
		displayName, description, alias, fieldsJSON,
	)
	if err != nil {
		return nil, "", err
	}

	namespace, err = wrapInFolderTree(fullPath, leaf)
	if err != nil {
		return nil, "", err
	}
	if err := validateNamespaceTree(namespace); err != nil {
		return nil, "", err
	}
	return namespace, fullPath, nil
}
