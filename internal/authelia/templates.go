package authelia

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type ClientTemplateCatalog struct {
	dir       string
	once      sync.Once
	templates []ClientTemplate
	err       error
}

type ClientTemplate struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	Client              Client `json:"client"`
	ApplicationMarkdown string `json:"applicationMarkdown"`
	SourcePath          string `json:"sourcePath"`
}

func NewClientTemplateCatalog(dir string) *ClientTemplateCatalog {
	return &ClientTemplateCatalog{dir: dir}
}

func (c *ClientTemplateCatalog) List() ([]ClientTemplate, error) {
	c.once.Do(func() {
		c.templates, c.err = loadClientTemplates(c.dir)
	})
	return c.templates, c.err
}

func loadClientTemplates(dir string) ([]ClientTemplate, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*", "index.md"))
	if err != nil {
		return nil, err
	}
	templates := []ClientTemplate{}
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		title := markdownTitle(string(data), filepath.Base(filepath.Dir(path)))
		appMarkdown := extractApplicationMarkdown(string(data))
		clients := extractTemplateClients(string(data))
		for _, client := range clients {
			id := filepath.Base(filepath.Dir(path)) + ":" + client.ClientID
			templates = append(templates, ClientTemplate{
				ID:                  id,
				Title:               templateTitle(title, client, len(clients) > 1),
				Client:              client,
				ApplicationMarkdown: appMarkdown,
				SourcePath:          path,
			})
		}
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Title < templates[j].Title })
	return templates, nil
}

func templateTitle(pageTitle string, client Client, multiClientPage bool) string {
	clientName := strings.TrimSpace(client.ClientName)
	if clientName != "" && (multiClientPage || !sameTitle(pageTitle, clientName)) {
		return clientName
	}
	if multiClientPage && client.ClientID != "" {
		return pageTitle + " (" + client.ClientID + ")"
	}
	return pageTitle
}

func sameTitle(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) ||
		strings.EqualFold(slugify(a), slugify(b))
}

func extractTemplateClients(markdown string) []Client {
	for _, block := range fencedYAMLBlocks(markdown) {
		if !strings.Contains(block, "identity_providers:") || !strings.Contains(block, "clients:") {
			continue
		}
		block = replaceDocVariables(block)
		var root yaml.Node
		if err := yaml.Unmarshal([]byte(block), &root); err != nil {
			continue
		}
		clients := getMappingPath(&root, "identity_providers", "oidc", "clients")
		if clients == nil || clients.Kind != yaml.SequenceNode {
			continue
		}
		out := make([]Client, 0, len(clients.Content))
		for _, item := range clients.Content {
			client := normalizeClient(Client{
				ClientID:                stringValue(item, "client_id"),
				ClientName:              stringValue(item, "client_name"),
				Public:                  boolValue(item, "public"),
				RedirectURIs:            stringSliceValue(item, "redirect_uris"),
				Scopes:                  stringSliceValue(item, "scopes"),
				GrantTypes:              stringSliceValue(item, "grant_types"),
				ResponseTypes:           stringSliceValue(item, "response_types"),
				AuthorizationPolicy:     stringValue(item, "authorization_policy"),
				RequirePKCE:             boolValue(item, "require_pkce"),
				TokenEndpointAuthMethod: stringValue(item, "token_endpoint_auth_method"),
				Extra:                   extraClientFields(item),
			})
			out = append(out, client)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func fencedYAMLBlocks(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	blocks := []string{}
	var current []string
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inBlock && (strings.HasPrefix(trimmed, "```yaml") || strings.HasPrefix(trimmed, "```yml")):
			inBlock = true
			current = nil
		case inBlock && strings.HasPrefix(trimmed, "```"):
			inBlock = false
			blocks = append(blocks, strings.Join(current, "\n"))
		case inBlock:
			current = append(current, line)
		}
	}
	return blocks
}

func markdownTitle(markdown, fallback string) string {
	re := regexp.MustCompile(`(?m)^title:\s*"?([^"\n]+)"?\s*$`)
	matches := re.FindStringSubmatch(markdown)
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return fallback
}

func extractApplicationMarkdown(markdown string) string {
	start := strings.Index(markdown, "### Application")
	if start == -1 {
		start = strings.Index(markdown, "## Application")
	}
	if start == -1 {
		return ""
	}
	end := strings.Index(markdown[start:], "\n## See Also")
	if end != -1 {
		markdown = markdown[start : start+end]
	} else {
		markdown = markdown[start:]
	}
	return strings.TrimSpace(replaceDocVariables(markdown))
}

var docVariableRE = regexp.MustCompile(`\{\{<\s*sitevar\s+name="[^"]+"\s+nojs="([^"]+)"\s*>\}\}`)

func replaceDocVariables(value string) string {
	value = docVariableRE.ReplaceAllString(value, "$1")
	value = strings.ReplaceAll(value, `{{< /callout >}}`, "")
	value = regexp.MustCompile(`\{\{<\s*callout[^>]*>\}\}`).ReplaceAllString(value, "")
	return value
}

func slugify(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "-")
	return value
}
