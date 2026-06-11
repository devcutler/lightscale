package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/devcutler/lightscale/daemon/crypto"
	"github.com/devcutler/lightscale/daemon/ipalloc"
	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/wire"
)

type userDTO = wire.User

func toUserDTO(u store.User) userDTO {
	return userDTO{
		ID: u.ID, Name: u.Name, Email: u.Email,
		PublicKey: u.PublicKey, PresharedKey: u.PresharedKey,
		IPAddress: u.IPAddress, Endpoint: u.Endpoint,
		Migrated:  u.PrivateKey == "",
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.deps.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		out = append(out, toUserDTO(u))
	}
	writeJSON(w, http.StatusOK, out)
}

type createUserReq = wire.CreateUserReq

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserReq
	if !readJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "name required"})
		return
	}

	ip := req.IP
	if ip == "" {
		taken, err := s.deps.Store.TakenUserIPs(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		ip, err = ipalloc.Allocate(s.deps.Config.WireGuard.ClientSubnet, taken,
			s.deps.Config.WireGuard.ServerIP)
		if err != nil {
			writeJSON(w, http.StatusConflict, errorBody{Error: err.Error()})
			return
		}
	} else {
		ok, err := ipalloc.IsInPrefix(s.deps.Config.WireGuard.ClientSubnet, ip)
		if err != nil || !ok {
			writeJSON(w, http.StatusBadRequest,
				errorBody{Error: fmt.Sprintf("ip %s not in client_subnet %s",
					ip, s.deps.Config.WireGuard.ClientSubnet)})
			return
		}
	}

	kp, err := crypto.GenerateKeypair()
	if err != nil {
		writeError(w, err)
		return
	}
	psk, err := crypto.GeneratePresharedKey()
	if err != nil {
		writeError(w, err)
		return
	}

	u, err := s.deps.Store.CreateUser(r.Context(), store.CreateUserInput{
		Name: req.Name, Email: req.Email,
		PublicKey: kp.PublicKey, PrivateKey: kp.PrivateKey, PresharedKey: psk,
		IPAddress: ip, Endpoint: req.Endpoint,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUserDTO(u))
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	u, err := s.deps.Store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u))
}

type updateUserReq = wire.UpdateUserReq

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req updateUserReq
	if !readJSON(w, r, &req) {
		return
	}
	u, err := s.deps.Store.UpdateUser(r.Context(), id, store.UpdateUserInput{
		Name: req.Name, Email: req.Email, Endpoint: req.Endpoint,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := s.deps.Store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getUserConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	u, err := s.deps.Store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	endpoint := u.Endpoint
	if endpoint == "" {
		endpoint = s.deps.Config.PublicEndpoint
	}
	if endpoint == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{
			Error: "no public_endpoint configured; set it in lightscale.toml or pass --endpoint on user create",
		})
		return
	}

	serverPub, err := s.deps.Store.GetSetting(r.Context(), "server_public_key")
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusServiceUnavailable,
			errorBody{Error: "server keypair not initialised; the daemon must be running at least once"})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}

	conf := renderClientConf(u, serverPub, endpoint, s.deps.Config.WireGuard.Subnet)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(conf))
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid id"})
		return 0, false
	}
	return id, true
}
