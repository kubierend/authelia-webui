package authelia

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSecretGenerator struct{}

func (fakeSecretGenerator) GenerateClientSecret() (SecretMaterial, error) {
	return SecretMaterial{
		Secret: "plain-generated-secret",
		Digest: "$pbkdf2-sha512$310000$salt$digest",
		Source: "test",
	}, nil
}

func (fakeSecretGenerator) HashClientSecret(secret string) (string, error) {
	return "$pbkdf2-sha512$310000$salt$digest", nil
}

func (fakeSecretGenerator) GenerateUserPassword() (SecretMaterial, error) {
	return SecretMaterial{
		Secret: "generated-password",
		Digest: "$argon2id$v=19$m=65536,t=3,p=4$salt$hash",
		Source: "test",
	}, nil
}

func TestClientCRUDWritesBlockYAML(t *testing.T) {
	dir := t.TempDir()
	usersFile := filepath.Join(dir, "users_database.yml")
	configFile := filepath.Join(dir, "configuration.yml")
	input := "identity_providers:\n    oidc:\n        clients: [{client_id: old, client_name: Old, public: false, redirect_uris: ['https://old.example/callback'], scopes: [openid, profile], grant_types: [authorization_code], response_types: [code], authorization_policy: two_factor, require_pkce: true, client_secret: $pbkdf2-sha512$310000$salt$digest, token_endpoint_auth_method: client_secret_basic}]\n"
	if err := os.WriteFile(configFile, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewStore(usersFile, configFile, fakeSecretGenerator{})
	created, err := store.CreateClient(CreateClientRequest{Client: Client{
		ClientID:            "new",
		ClientName:          "New",
		RedirectURIs:        []string{"https://new.example/callback"},
		Scopes:              []string{"openid", "profile", "email", "groups"},
		GrantTypes:          []string{"authorization_code"},
		ResponseTypes:       []string{"code"},
		AuthorizationPolicy: "two_factor",
		RequirePKCE:         true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if created.GeneratedClientSecret != "plain-generated-secret" {
		t.Fatalf("expected generated secret to be returned once, got %q", created.GeneratedClientSecret)
	}

	if _, err := store.UpdateClient("old", Client{
		ClientName:          "Updated",
		RedirectURIs:        []string{"https://updated.example/callback"},
		Scopes:              []string{"openid", "profile"},
		GrantTypes:          []string{"authorization_code"},
		ResponseTypes:       []string{"code"},
		AuthorizationPolicy: "two_factor",
		RequirePKCE:         true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteClient("new"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	if strings.Contains(output, "{client_id:") || strings.Contains(output, "clients: [") {
		t.Fatalf("expected block-style YAML, got:\n%s", output)
	}
	for _, expected := range []string{
		"clients:\n      - client_id: old",
		"client_name: Updated",
		"redirect_uris:\n          - https://updated.example/callback",
		"client_secret: $pbkdf2-sha512$310000$salt$digest",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestListUsersReadsDocumentRoot(t *testing.T) {
	dir := t.TempDir()
	usersFile := filepath.Join(dir, "users_database.yml")
	configFile := filepath.Join(dir, "configuration.yml")
	input := `users:
  alice:
    disabled: false
    displayname: Alice Example
    password: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
    email: alice@example.com
    groups:
      - admins
      - dev
`
	if err := os.WriteFile(usersFile, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewStore(usersFile, configFile, fakeSecretGenerator{})
	users, err := store.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "alice" || users[0].DisplayName != "Alice Example" || users[0].Email != "alice@example.com" {
		t.Fatalf("unexpected user: %#v", users[0])
	}
	if len(users[0].Groups) != 2 || users[0].Groups[0] != "admins" || users[0].Groups[1] != "dev" {
		t.Fatalf("unexpected groups: %#v", users[0].Groups)
	}
}

func TestUserCRUDPreservesPasswordAndCanReset(t *testing.T) {
	dir := t.TempDir()
	usersFile := filepath.Join(dir, "users_database.yml")
	configFile := filepath.Join(dir, "configuration.yml")

	store := NewStore(usersFile, configFile, fakeSecretGenerator{})
	created, err := store.CreateUser(CreateUserRequest{
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Groups:      []string{"admins"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.GeneratedPassword != "generated-password" {
		t.Fatalf("expected generated password to be returned once, got %q", created.GeneratedPassword)
	}

	updated, err := store.UpdateUser("alice", User{
		DisplayName: "Alice Updated",
		Email:       "alice.updated@example.com",
		Groups:      []string{"dev"},
		Disabled:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Disabled || updated.DisplayName != "Alice Updated" {
		t.Fatalf("unexpected updated user: %#v", updated)
	}
	data, err := os.ReadFile(usersFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "password: $argon2id$v=19$m=65536,t=3,p=4$salt$hash") {
		t.Fatalf("expected password hash to be preserved, got:\n%s", string(data))
	}

	reset, err := store.ResetUserPassword("alice")
	if err != nil {
		t.Fatal(err)
	}
	if reset.Secret != "generated-password" {
		t.Fatalf("unexpected reset material: %#v", reset)
	}

	if err := store.DeleteUser("alice"); err != nil {
		t.Fatal(err)
	}
	users, err := store.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("expected no users, got %#v", users)
	}
}
