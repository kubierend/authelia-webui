package authelia

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Store struct {
	usersFile       string
	configFile      string
	secretGenerator SecretGenerator
	mu              sync.Mutex
}

type User struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Groups      []string `json:"groups"`
	Disabled    bool     `json:"disabled"`
}

type CreateUserRequest struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Groups      []string `json:"groups"`
	Disabled    bool     `json:"disabled"`
}

type CreatedUser struct {
	User
	GeneratedPassword string `json:"generatedPassword,omitempty"`
}

type Client struct {
	ClientID                string         `json:"clientId"`
	ClientName              string         `json:"clientName"`
	Public                  bool           `json:"public"`
	RedirectURIs            []string       `json:"redirectUris"`
	Scopes                  []string       `json:"scopes"`
	GrantTypes              []string       `json:"grantTypes"`
	ResponseTypes           []string       `json:"responseTypes"`
	AuthorizationPolicy     string         `json:"authorizationPolicy"`
	RequirePKCE             bool           `json:"requirePkce"`
	TokenEndpointAuthMethod string         `json:"tokenEndpointAuthMethod,omitempty"`
	Extra                   map[string]any `json:"extra,omitempty"`
}

type CreatedClient struct {
	Client
	GeneratedClientSecret string `json:"generatedClientSecret,omitempty"`
}

type CreateClientRequest struct {
	Client
}

func NewStore(usersFile, configFile string, secretGenerator SecretGenerator) *Store {
	return &Store{usersFile: usersFile, configFile: configFile, secretGenerator: secretGenerator}
}

func (s *Store) ListUsers() ([]User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	root, err := readYAML(s.usersFile, defaultUsersRoot)
	if err != nil {
		return nil, err
	}
	users := getMappingPath(root, "users")
	if users == nil || users.Kind != yaml.MappingNode {
		return []User{}, nil
	}

	result := make([]User, 0, len(users.Content)/2)
	for i := 0; i < len(users.Content); i += 2 {
		username := users.Content[i].Value
		item := users.Content[i+1]
		result = append(result, User{
			Username:    username,
			DisplayName: stringValue(item, "displayname"),
			Email:       stringValue(item, "email"),
			Groups:      stringSliceValue(item, "groups"),
			Disabled:    boolValue(item, "disabled"),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Username < result[j].Username })
	return result, nil
}

func (s *Store) CreateUser(req CreateUserRequest) (CreatedUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user := normalizeUser(User{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Groups:      req.Groups,
		Disabled:    req.Disabled,
	})
	if user.Username == "" || user.Email == "" {
		return CreatedUser{}, errors.New("username and email are required")
	}

	root, err := readYAML(s.usersFile, defaultUsersRoot)
	if err != nil {
		return CreatedUser{}, err
	}
	users := ensureMappingPath(root, "users")
	if mappingValue(users, user.Username) != nil {
		return CreatedUser{}, fmt.Errorf("user %q already exists", user.Username)
	}

	material, err := s.secretGenerator.GenerateUserPassword()
	if err != nil {
		return CreatedUser{}, err
	}
	appendMapping(users, user.Username, userNode(user, material.Digest))
	sortMapping(users)

	if err := writeYAML(s.usersFile, root); err != nil {
		return CreatedUser{}, err
	}
	return CreatedUser{User: user, GeneratedPassword: material.Secret}, nil
}

func (s *Store) UpdateUser(username string, req User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username = strings.TrimSpace(username)
	user := normalizeUser(req)
	user.Username = username
	if user.Username == "" || user.Email == "" {
		return User{}, errors.New("username and email are required")
	}

	root, err := readYAML(s.usersFile, defaultUsersRoot)
	if err != nil {
		return User{}, err
	}
	users := ensureMappingPath(root, "users")
	current := mappingValue(users, username)
	if current == nil {
		return User{}, fmt.Errorf("user %q not found", username)
	}
	password := stringValue(current, "password")
	if password == "" {
		return User{}, fmt.Errorf("user %q is missing a password hash", username)
	}
	setMappingValue(users, username, userNode(user, password))
	sortMapping(users)
	if err := writeYAML(s.usersFile, root); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	root, err := readYAML(s.usersFile, defaultUsersRoot)
	if err != nil {
		return err
	}
	users := ensureMappingPath(root, "users")
	if !deleteMappingValue(users, username) {
		return fmt.Errorf("user %q not found", username)
	}
	return writeYAML(s.usersFile, root)
}

func (s *Store) ResetUserPassword(username string) (SecretMaterial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username = strings.TrimSpace(username)
	if username == "" {
		return SecretMaterial{}, errors.New("username is required")
	}
	root, err := readYAML(s.usersFile, defaultUsersRoot)
	if err != nil {
		return SecretMaterial{}, err
	}
	users := ensureMappingPath(root, "users")
	current := mappingValue(users, username)
	if current == nil {
		return SecretMaterial{}, fmt.Errorf("user %q not found", username)
	}
	material, err := s.secretGenerator.GenerateUserPassword()
	if err != nil {
		return SecretMaterial{}, err
	}
	setMappingValue(current, "password", scalarNode(material.Digest))
	if err := writeYAML(s.usersFile, root); err != nil {
		return SecretMaterial{}, err
	}
	return material, nil
}

func (s *Store) ListClients() ([]Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	root, err := readYAML(s.configFile, defaultConfigRoot)
	if err != nil {
		return nil, err
	}
	clients := getMappingPath(root, "identity_providers", "oidc", "clients")
	if clients == nil || clients.Kind != yaml.SequenceNode {
		return []Client{}, nil
	}

	result := make([]Client, 0, len(clients.Content))
	for _, item := range clients.Content {
		result = append(result, Client{
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
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ClientID < result[j].ClientID })
	return result, nil
}

func (s *Store) CreateClient(req CreateClientRequest) (CreatedClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := normalizeClient(req.Client)
	if client.ClientID == "" || client.ClientName == "" || len(client.RedirectURIs) == 0 {
		return CreatedClient{}, errors.New("client id, name, and redirect uris are required")
	}

	root, err := readYAML(s.configFile, defaultConfigRoot)
	if err != nil {
		return CreatedClient{}, err
	}
	clients := ensureSequencePath(root, "identity_providers", "oidc", "clients")
	for _, item := range clients.Content {
		if stringValue(item, "client_id") == client.ClientID {
			return CreatedClient{}, fmt.Errorf("client %q already exists", client.ClientID)
		}
	}

	var generatedSecret string
	clientSecretDigest := ""
	if !client.Public {
		material, err := s.secretGenerator.GenerateClientSecret()
		if err != nil {
			return CreatedClient{}, err
		}
		generatedSecret = material.Secret
		clientSecretDigest = material.Digest
	}

	clients.Content = append(clients.Content, clientNode(client, clientSecretDigest))

	if err := writeYAML(s.configFile, root); err != nil {
		return CreatedClient{}, err
	}
	return CreatedClient{Client: client, GeneratedClientSecret: generatedSecret}, nil
}

func (s *Store) UpdateClient(id string, req Client) (CreatedClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	client := normalizeClient(req)
	if id == "" {
		return CreatedClient{}, errors.New("client id is required")
	}
	client.ClientID = id
	if client.ClientName == "" || len(client.RedirectURIs) == 0 {
		return CreatedClient{}, errors.New("client name and redirect uris are required")
	}

	root, err := readYAML(s.configFile, defaultConfigRoot)
	if err != nil {
		return CreatedClient{}, err
	}
	clients := ensureSequencePath(root, "identity_providers", "oidc", "clients")
	index := clientIndex(clients, id)
	if index == -1 {
		return CreatedClient{}, fmt.Errorf("client %q not found", id)
	}

	var generatedSecret, digest string
	if !client.Public {
		digest = stringValue(clients.Content[index], "client_secret")
		if digest == "" {
			material, err := s.secretGenerator.GenerateClientSecret()
			if err != nil {
				return CreatedClient{}, err
			}
			generatedSecret = material.Secret
			digest = material.Digest
		}
	}
	clients.Content[index] = clientNode(client, digest)

	if err := writeYAML(s.configFile, root); err != nil {
		return CreatedClient{}, err
	}
	return CreatedClient{Client: client, GeneratedClientSecret: generatedSecret}, nil
}

func (s *Store) DeleteClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("client id is required")
	}
	root, err := readYAML(s.configFile, defaultConfigRoot)
	if err != nil {
		return err
	}
	clients := ensureSequencePath(root, "identity_providers", "oidc", "clients")
	index := clientIndex(clients, id)
	if index == -1 {
		return fmt.Errorf("client %q not found", id)
	}
	clients.Content = append(clients.Content[:index], clients.Content[index+1:]...)
	return writeYAML(s.configFile, root)
}

func (s *Store) RotateClientSecret(id string) (SecretMaterial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return SecretMaterial{}, errors.New("client id is required")
	}
	root, err := readYAML(s.configFile, defaultConfigRoot)
	if err != nil {
		return SecretMaterial{}, err
	}
	clients := ensureSequencePath(root, "identity_providers", "oidc", "clients")
	index := clientIndex(clients, id)
	if index == -1 {
		return SecretMaterial{}, fmt.Errorf("client %q not found", id)
	}
	if boolValue(clients.Content[index], "public") {
		return SecretMaterial{}, errors.New("public clients do not have client secrets")
	}

	material, err := s.secretGenerator.GenerateClientSecret()
	if err != nil {
		return SecretMaterial{}, err
	}
	setMappingValue(clients.Content[index], "client_secret", scalarNode(material.Digest))
	setMappingValue(clients.Content[index], "token_endpoint_auth_method", scalarNode("client_secret_basic"))
	if err := writeYAML(s.configFile, root); err != nil {
		return SecretMaterial{}, err
	}
	return material, nil
}

func readYAML(path string, fallback func() *yaml.Node) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fallback(), nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return fallback(), nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return &root, nil
}

func writeYAML(path string, root *yaml.Node) error {
	clearFlowStyle(root)
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		_ = encoder.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	data := buf.Bytes()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if stat, err := os.Stat(path); err == nil {
		mode = stat.Mode()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func cleanList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				out = append(out, part)
			}
		}
	}
	return out
}

func normalizeClient(client Client) Client {
	client.ClientID = strings.TrimSpace(client.ClientID)
	client.ClientName = strings.TrimSpace(client.ClientName)
	client.RedirectURIs = cleanList(client.RedirectURIs)
	client.Scopes = defaultList(cleanList(client.Scopes), []string{"openid", "profile", "email", "groups"})
	client.GrantTypes = defaultList(cleanList(client.GrantTypes), []string{"authorization_code"})
	client.ResponseTypes = defaultList(cleanList(client.ResponseTypes), []string{"code"})
	client.AuthorizationPolicy = defaultString(strings.TrimSpace(client.AuthorizationPolicy), "two_factor")
	client.TokenEndpointAuthMethod = strings.TrimSpace(client.TokenEndpointAuthMethod)
	client.Extra = cleanExtra(client.Extra)
	return client
}

func normalizeUser(user User) User {
	user.Username = strings.TrimSpace(user.Username)
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	user.Email = strings.TrimSpace(user.Email)
	user.Groups = cleanList(user.Groups)
	return user
}

func userNode(user User, passwordHash string) *yaml.Node {
	return mappingNode(
		"disabled", boolNode(user.Disabled),
		"displayname", scalarNode(user.DisplayName),
		"password", scalarNode(passwordHash),
		"email", scalarNode(user.Email),
		"groups", stringSequenceNode(user.Groups),
	)
}

func clientNode(client Client, clientSecretDigest string) *yaml.Node {
	node := mappingNode(
		"client_id", scalarNode(client.ClientID),
		"client_name", scalarNode(client.ClientName),
		"public", boolNode(client.Public),
		"redirect_uris", stringSequenceNode(client.RedirectURIs),
		"scopes", stringSequenceNode(client.Scopes),
		"grant_types", stringSequenceNode(client.GrantTypes),
		"response_types", stringSequenceNode(client.ResponseTypes),
		"authorization_policy", scalarNode(client.AuthorizationPolicy),
		"require_pkce", boolNode(client.RequirePKCE),
	)
	for key, value := range client.Extra {
		appendMapping(node, key, anyNode(value))
	}
	if client.Public {
		method := defaultString(client.TokenEndpointAuthMethod, "none")
		setMappingValue(node, "token_endpoint_auth_method", scalarNode(method))
		return node
	}
	appendMapping(node, "client_secret", scalarNode(clientSecretDigest))
	method := defaultString(client.TokenEndpointAuthMethod, "client_secret_basic")
	setMappingValue(node, "token_endpoint_auth_method", scalarNode(method))
	return node
}

func clientIndex(clients *yaml.Node, id string) int {
	for index, item := range clients.Content {
		if stringValue(item, "client_id") == id {
			return index
		}
	}
	return -1
}

func defaultList(values, fallback []string) []string {
	if len(values) == 0 {
		return fallback
	}
	return values
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
