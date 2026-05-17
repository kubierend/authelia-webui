package authelia

import (
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"
)

func defaultUsersRoot() *yaml.Node {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	doc.Content = []*yaml.Node{mappingNode("users", &yaml.Node{Kind: yaml.MappingNode})}
	return doc
}

func defaultConfigRoot() *yaml.Node {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	doc.Content = []*yaml.Node{&yaml.Node{Kind: yaml.MappingNode}}
	return doc
}

func documentMapping(root *yaml.Node) *yaml.Node {
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			root.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		return root.Content[0]
	}
	return root
}

func getMappingPath(root *yaml.Node, path ...string) *yaml.Node {
	current := documentMapping(root)
	for _, key := range path {
		current = mappingValue(current, key)
		if current == nil {
			return nil
		}
	}
	return current
}

func ensureMappingPath(root *yaml.Node, path ...string) *yaml.Node {
	current := documentMapping(root)
	for _, key := range path {
		next := mappingValue(current, key)
		if next == nil {
			next = &yaml.Node{Kind: yaml.MappingNode}
			appendMapping(current, key, next)
		}
		if next.Kind != yaml.MappingNode {
			next.Kind = yaml.MappingNode
			next.Content = nil
			next.Value = ""
		}
		current = next
	}
	return current
}

func ensureSequencePath(root *yaml.Node, path ...string) *yaml.Node {
	parentPath := path[:len(path)-1]
	key := path[len(path)-1]
	parent := ensureMappingPath(root, parentPath...)
	next := mappingValue(parent, key)
	if next == nil {
		next = &yaml.Node{Kind: yaml.SequenceNode}
		appendMapping(parent, key, next)
	}
	if next.Kind != yaml.SequenceNode {
		next.Kind = yaml.SequenceNode
		next.Content = nil
		next.Value = ""
	}
	return next
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	appendMapping(node, key, value)
}

func deleteMappingValue(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return true
		}
	}
	return false
}

func appendMapping(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content, scalarNode(key), value)
}

func sortMapping(node *yaml.Node) {
	type pair struct {
		key   *yaml.Node
		value *yaml.Node
	}
	pairs := make([]pair, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		pairs = append(pairs, pair{key: node.Content[i], value: node.Content[i+1]})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].key.Value < pairs[j].key.Value })
	node.Content = node.Content[:0]
	for _, pair := range pairs {
		node.Content = append(node.Content, pair.key, pair.value)
	}
}

func mappingNode(values ...any) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(values); i += 2 {
		appendMapping(node, values[i].(string), values[i+1].(*yaml.Node))
	}
	return node
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func boolNode(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}
}

func stringSequenceNode(values []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode}
	for _, value := range values {
		node.Content = append(node.Content, scalarNode(value))
	}
	return node
}

func stringValue(node *yaml.Node, key string) string {
	value := mappingValue(node, key)
	if value == nil {
		return ""
	}
	return value.Value
}

func boolValue(node *yaml.Node, key string) bool {
	value := mappingValue(node, key)
	return value != nil && value.Value == "true"
}

func stringSliceValue(node *yaml.Node, key string) []string {
	value := mappingValue(node, key)
	if value == nil || value.Kind != yaml.SequenceNode {
		return []string{}
	}
	out := make([]string, 0, len(value.Content))
	for _, item := range value.Content {
		out = append(out, item.Value)
	}
	return out
}

var knownClientFields = map[string]bool{
	"client_id":                  true,
	"client_name":                true,
	"client_secret":              true,
	"public":                     true,
	"redirect_uris":              true,
	"scopes":                     true,
	"grant_types":                true,
	"response_types":             true,
	"authorization_policy":       true,
	"require_pkce":               true,
	"token_endpoint_auth_method": true,
}

func extraClientFields(node *yaml.Node) map[string]any {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	extra := map[string]any{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if knownClientFields[key] {
			continue
		}
		extra[key] = nodeToAny(node.Content[i+1])
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

func cleanExtra(extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return nil
	}
	cleaned := map[string]any{}
	for key, value := range extra {
		if key == "" || knownClientFields[key] || value == nil {
			continue
		}
		cleaned[key] = value
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func nodeToAny(node *yaml.Node) any {
	var value any
	if err := node.Decode(&value); err != nil {
		return node.Value
	}
	return value
}

func anyNode(value any) *yaml.Node {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return scalarNode("")
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return &node
}

func clearFlowStyle(node *yaml.Node) {
	if node == nil {
		return
	}
	node.Style = 0
	for _, child := range node.Content {
		clearFlowStyle(child)
	}
}
