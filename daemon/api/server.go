package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devcutler/lightscale/daemon/docker"
	"github.com/devcutler/lightscale/daemon/proxy"
	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/config"
	"github.com/devcutler/lightscale/shared/origin"
	"github.com/devcutler/lightscale/shared/wire"
)

type StatusProvider interface {
	Snapshot() StatusSnapshot
}

type StatusSnapshot = wire.StatusSnapshot
type PeersProvider interface {
	PeerStatus() ([]PeerStatus, error)
}

type FlowSnapshot struct {
	ID         uint64
	SrcUserID  int64
	ObjectType string
	ObjectID   int64
	Port       int
	Protocol   string
}
type FlowsProvider interface {
	Snapshot() []FlowSnapshot
}

type UserBrief struct {
	ID        int64
	Name      string
	IP        string
	PublicKey string
}

type ServiceBrief struct {
	ID   int64
	Name string
	IP   string
}

type Resolver interface {
	Users() []UserBrief
	UserByID(id int64) (UserBrief, bool)
	ServiceByID(id int64) (ServiceBrief, bool)
}

type PeerStatus struct {
	PublicKey         string
	PresharedKey      string
	AllowedIPs        []string
	Endpoint          string
	LastHandshake     time.Time
	KeepaliveInterval int
	RxBytes           uint64
	TxBytes           uint64
}

type OriginChecker interface {
	Resolve(ctx context.Context, spec origin.Spec, port int, proto string) (proxy.Target, error)
}

type Deps struct {
	Store         *store.Store
	Config        *config.Config
	Docker        *docker.Client
	Status        StatusProvider
	Peers         PeersProvider
	Flows         FlowsProvider
	Resolver      Resolver
	OriginChecker OriginChecker
	Now           func() time.Time
}
type Server struct {
	deps Deps
	mux  *chi.Mux

	dockerMu sync.RWMutex
	docker   *docker.Client
}

func New(deps Deps) *Server {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	s := &Server{deps: deps, mux: chi.NewRouter(), docker: deps.Docker}
	s.mux.Use(middleware.Recoverer)
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/status", s.handleStatus)

		r.Route("/users", func(r chi.Router) {
			r.Get("/", s.listUsers)
			r.Post("/", s.createUser)
			r.Get("/{id}", s.getUser)
			r.Patch("/{id}", s.updateUser)
			r.Delete("/{id}", s.deleteUser)
			r.Get("/{id}/config", s.getUserConfig)
		})

		r.Route("/user-groups", func(r chi.Router) {
			r.Get("/", s.listUserGroups)
			r.Post("/", s.createUserGroup)
			r.Get("/{id}", s.getUserGroup)
			r.Patch("/{id}", s.updateUserGroup)
			r.Delete("/{id}", s.deleteUserGroup)
			r.Get("/{id}/members", s.listUserGroupMembers)
			r.Post("/{id}/members", s.addUserGroupMember)
			r.Delete("/{id}/members/{uid}", s.removeUserGroupMember)
		})

		r.Route("/services", func(r chi.Router) {
			r.Get("/", s.listServices)
			r.Post("/", s.createService)
			r.Get("/{id}", s.getService)
			r.Patch("/{id}", s.updateService)
			r.Delete("/{id}", s.deleteService)
			r.Post("/{id}/check", s.checkServiceOrigin)
		})

		r.Route("/service-groups", func(r chi.Router) {
			r.Get("/", s.listServiceGroups)
			r.Post("/", s.createServiceGroup)
			r.Get("/{id}", s.getServiceGroup)
			r.Patch("/{id}", s.updateServiceGroup)
			r.Delete("/{id}", s.deleteServiceGroup)
			r.Get("/{id}/members", s.listServiceGroupMembers)
			r.Post("/{id}/members", s.addServiceGroupMember)
			r.Delete("/{id}/members/{sid}", s.removeServiceGroupMember)
		})

		r.Route("/policies", func(r chi.Router) {
			r.Get("/", s.listPolicies)
			r.Post("/", s.createPolicy)
			r.Delete("/{id}", s.deletePolicy)
		})

		r.Get("/principals", s.listPrincipals)
		r.Get("/objects", s.listObjects)
		r.Get("/dns", s.getDNSZone)
		r.Get("/discover/containers", s.discoverContainers)
		r.Get("/peers", s.listPeers)
		r.Get("/connections", s.listConnections)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{Error: err.Error()})
	case errors.Is(err, store.ErrNameTaken),
		errors.Is(err, store.ErrIPInUse),
		errors.Is(err, store.ErrPortConflict):
		writeJSON(w, http.StatusConflict, errorBody{Error: err.Error()})
	case errors.Is(err, store.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
	}
}
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("invalid JSON: %v", err)})
		return false
	}
	return true
}

type errorBody = wire.Error

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Status.Snapshot())
}

type principalDTO = wire.Principal

func (s *Server) listPrincipals(w http.ResponseWriter, r *http.Request) {
	users, err := s.deps.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	groups, err := s.deps.Store.ListUserGroups(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]principalDTO, 0, len(users)+len(groups))
	for _, u := range users {
		out = append(out, principalDTO{Type: "user", ID: u.ID, Name: u.Name})
	}
	for _, g := range groups {
		out = append(out, principalDTO{Type: "user_group", ID: g.ID, Name: g.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

type objectDTO = wire.Object

func (s *Server) listObjects(w http.ResponseWriter, r *http.Request) {
	out := []objectDTO{}
	users, err := s.deps.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for _, u := range users {
		out = append(out, objectDTO{Type: "user", ID: u.ID, Name: u.Name})
	}
	userGroups, err := s.deps.Store.ListUserGroups(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for _, g := range userGroups {
		out = append(out, objectDTO{Type: "user_group", ID: g.ID, Name: g.Name})
	}
	services, err := s.deps.Store.ListServices(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for _, sv := range services {
		out = append(out, objectDTO{Type: "service", ID: sv.ID, Name: sv.Name})
	}
	serviceGroups, err := s.deps.Store.ListServiceGroups(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for _, g := range serviceGroups {
		out = append(out, objectDTO{Type: "service_group", ID: g.ID, Name: g.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) SetDocker(d *docker.Client) {
	s.dockerMu.Lock()
	s.docker = d
	s.dockerMu.Unlock()
}

func (s *Server) discoverContainers(w http.ResponseWriter, r *http.Request) {
	s.dockerMu.RLock()
	dockerClient := s.docker
	s.dockerMu.RUnlock()
	if dockerClient == nil {
		writeJSON(w, http.StatusOK, []docker.ContainerSummary{})
		return
	}
	containers, err := dockerClient.ListContainers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, containers)
}
