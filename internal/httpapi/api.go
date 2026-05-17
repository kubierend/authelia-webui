package httpapi

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/kubierend/authelia-webui/internal/authelia"
)

type Config struct {
	ListenAddr      string
	UsersFile       string
	ConfigFile      string
	AutheliaBinary  string
	DocsClientsDir  string
	SecretGenerator string
}

type store interface {
	ListUsers() ([]authelia.User, error)
	CreateUser(authelia.CreateUserRequest) (authelia.CreatedUser, error)
	UpdateUser(string, authelia.User) (authelia.User, error)
	DeleteUser(string) error
	ResetUserPassword(string) (authelia.SecretMaterial, error)
	ListClients() ([]authelia.Client, error)
	CreateClient(authelia.CreateClientRequest) (authelia.CreatedClient, error)
	UpdateClient(string, authelia.Client) (authelia.CreatedClient, error)
	DeleteClient(string) error
	RotateClientSecret(string) (authelia.SecretMaterial, error)
}

func NewRouter(cfg Config, store store, static embed.FS) http.Handler {
	mux := http.NewServeMux()
	templateCatalog := authelia.NewClientTemplateCatalog(cfg.DocsClientsDir)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"usersFile":       cfg.UsersFile,
			"configFile":      cfg.ConfigFile,
			"autheliaBinary":  cfg.AutheliaBinary,
			"docsClientsDir":  cfg.DocsClientsDir,
			"secretGenerator": cfg.SecretGenerator,
		})
	})
	mux.HandleFunc("GET /api/client-templates", func(w http.ResponseWriter, r *http.Request) {
		templates, err := templateCatalog.List()
		respond(w, templates, err)
	})
	mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
		users, err := store.ListUsers()
		respond(w, users, err)
	})
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		var req authelia.CreateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		user, err := store.CreateUser(req)
		respondCreated(w, user, err)
	})
	mux.HandleFunc("PUT /api/users/{username}", func(w http.ResponseWriter, r *http.Request) {
		var req authelia.User
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		user, err := store.UpdateUser(r.PathValue("username"), req)
		respond(w, user, err)
	})
	mux.HandleFunc("DELETE /api/users/{username}", func(w http.ResponseWriter, r *http.Request) {
		if err := store.DeleteUser(r.PathValue("username")); err != nil {
			writeError(w, statusFor(err), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/users/{username}/password", func(w http.ResponseWriter, r *http.Request) {
		password, err := store.ResetUserPassword(r.PathValue("username"))
		respondCreated(w, password, err)
	})
	mux.HandleFunc("GET /api/clients", func(w http.ResponseWriter, r *http.Request) {
		clients, err := store.ListClients()
		respond(w, clients, err)
	})
	mux.HandleFunc("POST /api/clients", func(w http.ResponseWriter, r *http.Request) {
		var req authelia.CreateClientRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		client, err := store.CreateClient(req)
		respondCreated(w, client, err)
	})
	mux.HandleFunc("PUT /api/clients/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req authelia.Client
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		client, err := store.UpdateClient(r.PathValue("id"), req)
		respond(w, client, err)
	})
	mux.HandleFunc("DELETE /api/clients/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := store.DeleteClient(r.PathValue("id")); err != nil {
			writeError(w, statusFor(err), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/clients/{id}/secret", func(w http.ResponseWriter, r *http.Request) {
		secret, err := store.RotateClientSecret(r.PathValue("id"))
		respondCreated(w, secret, err)
	})
	assets, err := fs.Sub(static, "static")
	if err == nil {
		mux.Handle("/", spaHandler(http.FS(assets)))
	}
	return mux
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func respondCreated(w http.ResponseWriter, data any, err error) {
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, data)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func statusFor(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := err.Error()
	if strings.Contains(message, "already exists") {
		return http.StatusConflict
	}
	if strings.Contains(message, "not found") {
		return http.StatusNotFound
	}
	if strings.Contains(message, "required") || errors.Is(err, fs.ErrInvalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func spaHandler(files http.FileSystem) http.Handler {
	fileServer := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		file, err := files.Open(path)
		if err == nil {
			_ = file.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
