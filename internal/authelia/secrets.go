package authelia

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	clientSecretLength = 72
	userPasswordLength = 24
)

type SecretMaterial struct {
	Secret string `json:"secret"`
	Digest string `json:"digest"`
	Source string `json:"source"`
}

type SecretGenerator interface {
	GenerateClientSecret() (SecretMaterial, error)
	HashClientSecret(secret string) (string, error)
	GenerateUserPassword() (SecretMaterial, error)
}

type CLISecretGenerator struct {
	path string
}

func NewCLISecretGenerator(path string) CLISecretGenerator {
	return CLISecretGenerator{path: path}
}

func (g CLISecretGenerator) GenerateClientSecret() (SecretMaterial, error) {
	args := []string{
		"crypto", "hash", "generate", "pbkdf2",
		"--random",
		"--random.length", fmt.Sprintf("%d", clientSecretLength),
		"--random.charset", "alphanumeric",
		"--variant", "sha512",
		"--no-confirm",
	}
	output, err := g.run(args...)
	if err == nil {
		secret := lineValue(output, "Random Password:")
		digest := lineValue(output, "Digest:")
		if secret != "" && digest != "" {
			return SecretMaterial{Secret: secret, Digest: digest, Source: "authelia-cli"}, nil
		}
		err = fmt.Errorf("unexpected authelia output: %s", strings.TrimSpace(output))
	}
	return SecretMaterial{}, err
}

func (g CLISecretGenerator) HashClientSecret(secret string) (string, error) {
	output, err := g.run(
		"crypto", "hash", "generate", "pbkdf2",
		"--password", secret,
		"--variant", "sha512",
		"--no-confirm",
	)
	if err == nil {
		digest := lineValue(output, "Digest:")
		if digest != "" {
			return digest, nil
		}
		err = fmt.Errorf("unexpected authelia output: %s", strings.TrimSpace(output))
	}
	return "", err
}

func (g CLISecretGenerator) GenerateUserPassword() (SecretMaterial, error) {
	output, err := g.run(
		"crypto", "hash", "generate", "argon2",
		"--random",
		"--random.length", fmt.Sprintf("%d", userPasswordLength),
		"--random.charset", "alphanumeric",
		"--variant", "argon2id",
		"--no-confirm",
	)
	if err == nil {
		secret := lineValue(output, "Random Password:")
		digest := lineValue(output, "Digest:")
		if secret != "" && digest != "" {
			return SecretMaterial{Secret: secret, Digest: digest, Source: "authelia-cli"}, nil
		}
		err = fmt.Errorf("unexpected authelia output: %s", strings.TrimSpace(output))
	}
	return SecretMaterial{}, err
}

func (g CLISecretGenerator) run(args ...string) (string, error) {
	cmd := exec.Command(g.path, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return out.String(), err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return out.String(), err
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		return out.String(), fmt.Errorf("authelia command timed out")
	}
}

func lineValue(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
