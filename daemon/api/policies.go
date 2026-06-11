package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/wire"
)

type policyDTO = wire.Policy

func (s *Server) listPolicies(w http.ResponseWriter, r *http.Request) {
	rules, err := s.deps.Store.ListPolicies(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]policyDTO, 0, len(rules))
	for _, p := range rules {
		dto := policyDTO{
			ID: p.ID, SubjectType: p.SubjectType, SubjectID: p.SubjectID,
			ObjectType: p.ObjectType, ObjectID: p.ObjectID, Action: p.Action,
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		}
		dto.SubjectName, _ = nameFor(r.Context(), s.deps.Store, p.SubjectType, p.SubjectID)
		dto.ObjectName, _ = nameFor(r.Context(), s.deps.Store, p.ObjectType, p.ObjectID)
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

type createPolicyReq = wire.CreatePolicyReq

func (s *Server) createPolicy(w http.ResponseWriter, r *http.Request) {
	var req createPolicyReq
	if !readJSON(w, r, &req) {
		return
	}

	subjectType, subjectID, err := resolveSubject(r.Context(), s.deps.Store, req.SubjectType, req.SubjectID, req.SubjectName)
	if err != nil {
		writeError(w, err)
		return
	}
	objectType, objectID, err := resolveObject(r.Context(), s.deps.Store, req.ObjectType, req.ObjectID, req.ObjectName)
	if err != nil {
		writeError(w, err)
		return
	}

	rule, err := s.deps.Store.CreatePolicy(r.Context(), store.CreatePolicyInput{
		SubjectType: subjectType, SubjectID: subjectID,
		ObjectType: objectType, ObjectID: objectID,
		Action: req.Action,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	dto := policyDTO{
		ID: rule.ID, SubjectType: rule.SubjectType, SubjectID: rule.SubjectID,
		ObjectType: rule.ObjectType, ObjectID: rule.ObjectID, Action: rule.Action,
		CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}
	dto.SubjectName, _ = nameFor(r.Context(), s.deps.Store, rule.SubjectType, rule.SubjectID)
	dto.ObjectName, _ = nameFor(r.Context(), s.deps.Store, rule.ObjectType, rule.ObjectID)
	writeJSON(w, http.StatusCreated, dto)
}

func (s *Server) deletePolicy(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := s.deps.Store.DeletePolicy(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func resolveSubject(ctx context.Context, st *store.Store, t string, id int64, name string) (string, int64, error) {
	if id != 0 && t != "" {
		return t, id, nil
	}
	if name == "" {
		return "", 0, fmt.Errorf("%w: subject required", store.ErrInvalidInput)
	}
	if u, err := st.GetUserByName(ctx, name); err == nil {
		return "user", u.ID, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", 0, err
	}
	if g, err := st.GetUserGroupByName(ctx, name); err == nil {
		return "user_group", g.ID, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", 0, err
	}
	return "", 0, fmt.Errorf("%w: subject %q", store.ErrNotFound, name)
}

func resolveObject(ctx context.Context, st *store.Store, t string, id int64, name string) (string, int64, error) {
	if id != 0 && t != "" {
		return t, id, nil
	}
	if name == "" {
		return "", 0, fmt.Errorf("%w: object required", store.ErrInvalidInput)
	}
	if sv, err := st.GetServiceByName(ctx, name); err == nil {
		return "service", sv.ID, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", 0, err
	}
	if g, err := st.GetServiceGroupByName(ctx, name); err == nil {
		return "service_group", g.ID, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", 0, err
	}
	if u, err := st.GetUserByName(ctx, name); err == nil {
		return "user", u.ID, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", 0, err
	}
	if g, err := st.GetUserGroupByName(ctx, name); err == nil {
		return "user_group", g.ID, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", 0, err
	}
	return "", 0, fmt.Errorf("%w: object %q", store.ErrNotFound, name)
}

func nameFor(ctx context.Context, st *store.Store, t string, id int64) (string, error) {
	switch t {
	case "user":
		u, err := st.GetUser(ctx, id)
		return u.Name, err
	case "user_group":
		g, err := st.GetUserGroup(ctx, id)
		return g.Name, err
	case "service":
		sv, err := st.GetService(ctx, id)
		return sv.Name, err
	case "service_group":
		g, err := st.GetServiceGroup(ctx, id)
		return g.Name, err
	}
	panic("nameFor: unreachable type " + t)
}
