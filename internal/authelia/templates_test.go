package authelia

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientTemplates(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "grafana", `---
title: "Grafana"
---

## Configuration

`+"```yaml"+`
identity_providers:
  oidc:
    clients:
      - client_id: grafana
        client_name: Grafana
        public: false
        authorization_policy: two_factor
        require_pkce: true
        pkce_challenge_method: S256
        scopes:
          - openid
          - profile
        redirect_uris:
          - https://grafana.example.com/login/generic_oauth
        response_types:
          - code
        grant_types:
          - authorization_code
        token_endpoint_auth_method: client_secret_basic
`+"```"+`

### Application

Configure Grafana with the generated client secret.
`)

	writeTemplate(t, dir, "ocis", `---
title: "ownCloud Infinite Scale"
---

## Configuration

`+"```yaml"+`
identity_providers:
  oidc:
    clients:
      - client_id: ocis
        client_name: ownCloud Infinite Scale
        public: true
        authorization_policy: two_factor
        require_pkce: true
        scopes: [openid, profile]
        redirect_uris: [https://owncloud.example.com/oidc-callback.html]
        response_types: [code]
        grant_types: [authorization_code]
        token_endpoint_auth_method: none
      - client_id: e4rAsNUSIUs0lF4nbv9FmCeUkTlV9GdgTLDH1b5uie7syb90SzEVrbN7HIpmWJeD
        client_name: ownCloud Infinite Scale (Android)
        public: false
        authorization_policy: two_factor
        require_pkce: true
        scopes: [openid, profile]
        redirect_uris: [oc://android.owncloud.com]
        response_types: [code]
        grant_types: [authorization_code]
        token_endpoint_auth_method: client_secret_basic
`+"```"+`

### Application

Configure ownCloud with the generated values.
`)

	templates, err := loadClientTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(templates))
	}

	grafana := findTemplate(templates, "grafana")
	if grafana == nil {
		t.Fatal("expected grafana template")
	}
	if grafana.Client.ClientName != "Grafana" {
		t.Fatalf("unexpected grafana client name: %q", grafana.Client.ClientName)
	}
	if grafana.Client.Extra["pkce_challenge_method"] != "S256" {
		t.Fatalf("expected grafana pkce_challenge_method extra field, got %#v", grafana.Client.Extra)
	}
	if grafana.ApplicationMarkdown == "" {
		t.Fatal("expected grafana application instructions")
	}

	ocisAndroid := findTemplate(templates, "e4rAsNUSIUs0lF4nbv9FmCeUkTlV9GdgTLDH1b5uie7syb90SzEVrbN7HIpmWJeD")
	if ocisAndroid == nil {
		t.Fatal("expected oCIS android template")
	}
	if ocisAndroid.Title != "ownCloud Infinite Scale (Android)" {
		t.Fatalf("unexpected oCIS android template title: %q", ocisAndroid.Title)
	}
}

func writeTemplate(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findTemplate(templates []ClientTemplate, clientID string) *ClientTemplate {
	for i := range templates {
		if templates[i].Client.ClientID == clientID {
			return &templates[i]
		}
	}
	return nil
}
