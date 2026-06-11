package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/wire"
)

type userGroupDTO = wire.UserGroup

func toUserGroupDTO(g store.UserGroup) userGroupDTO {
	return userGroupDTO{
		ID: g.ID, Name: g.Name, LANMode: g.LANMode,
		CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

type createUserGroupReq = wire.CreateUserGroupReq

func (s *Server) listUserGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.deps.Store.ListUserGroups(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]userGroupDTO, 0, len(groups))
	for _, g := range groups {
		out = append(out, toUserGroupDTO(g))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createUserGroup(w http.ResponseWriter, r *http.Request) {
	var req createUserGroupReq
	if !readJSON(w, r, &req) {
		return
	}
	g, err := s.deps.Store.CreateUserGroup(r.Context(), req.Name, req.LANMode)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUserGroupDTO(g))
}

type updateUserGroupReq = wire.UpdateUserGroupReq

func (s *Server) updateUserGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req updateUserGroupReq
	if !readJSON(w, r, &req) {
		return
	}
	g, err := s.deps.Store.UpdateUserGroup(r.Context(), id, store.UpdateUserGroupInput{
		Name: req.Name, LANMode: req.LANMode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserGroupDTO(g))
}

func (s *Server) getUserGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	g, err := s.deps.Store.GetUserGroup(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserGroupDTO(g))
}

func (s *Server) deleteUserGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := s.deps.Store.DeleteUserGroup(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listUserGroupMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	users, err := s.deps.Store.UserGroupMembers(r.Context(), id)
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

type userGroupMemberReq = wire.UserGroupMemberReq

func (s *Server) addUserGroupMember(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req userGroupMemberReq
	if !readJSON(w, r, &req) {
		return
	}
	uid := req.UserID
	if uid == 0 && req.UserName != "" {
		u, err := s.deps.Store.GetUserByName(r.Context(), req.UserName)
		if err != nil {
			writeError(w, err)
			return
		}
		uid = u.ID
	}
	if uid == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "user_id or user_name required"})
		return
	}
	if err := s.deps.Store.AddUserToGroup(r.Context(), id, uid); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeUserGroupMember(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uidRaw := chi.URLParam(r, "uid")
	uid, err := strconv.ParseInt(uidRaw, 10, 64)
	if err != nil || uid <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid uid"})
		return
	}
	if err := s.deps.Store.RemoveUserFromGroup(r.Context(), id, uid); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type serviceGroupDTO = wire.ServiceGroup

func toServiceGroupDTO(g store.ServiceGroup) serviceGroupDTO {
	return serviceGroupDTO{
		ID: g.ID, Name: g.Name, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

type createServiceGroupReq = wire.CreateServiceGroupReq

func (s *Server) listServiceGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.deps.Store.ListServiceGroups(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]serviceGroupDTO, 0, len(groups))
	for _, g := range groups {
		out = append(out, toServiceGroupDTO(g))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createServiceGroup(w http.ResponseWriter, r *http.Request) {
	var req createServiceGroupReq
	if !readJSON(w, r, &req) {
		return
	}
	g, err := s.deps.Store.CreateServiceGroup(r.Context(), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toServiceGroupDTO(g))
}

func (s *Server) getServiceGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	g, err := s.deps.Store.GetServiceGroup(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toServiceGroupDTO(g))
}

type updateServiceGroupReq = wire.UpdateServiceGroupReq

func (s *Server) updateServiceGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req updateServiceGroupReq
	if !readJSON(w, r, &req) {
		return
	}
	g, err := s.deps.Store.UpdateServiceGroup(r.Context(), id, store.UpdateServiceGroupInput{Name: req.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toServiceGroupDTO(g))
}

func (s *Server) deleteServiceGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := s.deps.Store.DeleteServiceGroup(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listServiceGroupMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	services, err := s.deps.Store.ServiceGroupMembers(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]serviceDTO, 0, len(services))
	for _, sv := range services {
		out = append(out, toServiceDTO(sv))
	}
	writeJSON(w, http.StatusOK, out)
}

type serviceGroupMemberReq = wire.ServiceGroupMemberReq

func (s *Server) addServiceGroupMember(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req serviceGroupMemberReq
	if !readJSON(w, r, &req) {
		return
	}
	sid := req.ServiceID
	if sid == 0 && req.ServiceName != "" {
		sv, err := s.deps.Store.GetServiceByName(r.Context(), req.ServiceName)
		if err != nil {
			writeError(w, err)
			return
		}
		sid = sv.ID
	}
	if sid == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "service_id or service_name required"})
		return
	}
	if err := s.deps.Store.AddServiceToGroup(r.Context(), id, sid); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeServiceGroupMember(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	sidRaw := chi.URLParam(r, "sid")
	sid, err := strconv.ParseInt(sidRaw, 10, 64)
	if err != nil || sid <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid sid"})
		return
	}
	if err := s.deps.Store.RemoveServiceFromGroup(r.Context(), id, sid); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
