package main

import (
	"embed"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/kubierend/authelia-webui/internal/authelia"
	"github.com/kubierend/authelia-webui/internal/httpapi"
)

//go:embed all:static
var staticFiles embed.FS

func main() {
	cfg := httpapi.Config{
		ListenAddr:      os.Getenv("LISTEN_ADDR"),
		UsersFile:       envOrDefault("AUTHELIA_USERS_FILE", "/config/users_database.yml"),
		ConfigFile:      envOrDefault("AUTHELIA_CONFIG_FILE", "/config/configuration.yml"),
		AutheliaBinary:  envOrDefault("AUTHELIA_BINARY", defaultAutheliaBinary()),
		DocsClientsDir:  envOrDefault("AUTHELIA_DOCS_CLIENTS_DIR", "./authelia-src/docs/content/integration/openid-connect/clients"),
		SecretGenerator: "authelia-cli",
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}

	generator := authelia.NewCLISecretGenerator(cfg.AutheliaBinary)
	store := authelia.NewStore(cfg.UsersFile, cfg.ConfigFile, generator)
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           httpapi.NewRouter(cfg, store, staticFiles),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting authelia webui", "addr", cfg.ListenAddr, "users_file", cfg.UsersFile, "config_file", cfg.ConfigFile)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func defaultAutheliaBinary() string {
	if _, err := os.Stat("./authelia"); err == nil {
		return "./authelia"
	}
	return "authelia"
}
