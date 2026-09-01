package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/devcutler/lightscale/daemon/ipalloc"
	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/origin"
	"github.com/devcutler/lightscale/shared/wire"
)

type serviceDTO = wire.Service

type portDTO = wire.ServicePort

func toServiceDTO(sv store.Service) serviceDTO {
	ports := make([]portDTO, 0, len(sv.Ports))
	for _, p := range sv.Ports {
		ports = append(ports, portDTO{Port: p.Port, Protocol: p.Protocol})
	}
	return serviceDTO{
		ID: sv.ID, Name: sv.Name, Hostname: sv.Hostname,
		OriginKind: string(sv.Origin.Kind), OriginValue: sv.Origin.Value,
		OriginNetwork: sv.Origin.Network,
		IPAddress:     sv.IPAddress, Description: sv.Description, Ports: ports,
		CreatedAt: sv.CreatedAt, UpdatedAt: sv.UpdatedAt,
	}
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.deps.Store.ListServices(r.Context())
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

type createServiceReq = wire.CreateServiceReq

func (s *Server) createService(w http.ResponseWriter, r *http.Request) {
	var req createServiceReq
	if !readJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "name required"})
		return
	}
	spec, err := origin.Validate(origin.Spec{
		Kind:    origin.Kind(req.OriginKind),
		Value:   req.OriginValue,
		Network: req.OriginNetwork,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}

	hostname := req.Hostname
	if hostname == "" {
		hostname = fmt.Sprintf("%s.%s", req.Name, s.deps.Config.Domain)
	}

	ip := req.IP
	if ip == "" {
		taken, terr := s.deps.Store.TakenServiceIPs(r.Context())
		if terr != nil {
			writeError(w, terr)
			return
		}
		ip, err = ipalloc.Allocate(s.deps.Config.WireGuard.ServiceSubnet, taken)
		if err != nil {
			writeJSON(w, http.StatusConflict, errorBody{Error: err.Error()})
			return
		}
	} else {
		ok, err := ipalloc.IsInPrefix(s.deps.Config.WireGuard.ServiceSubnet, ip)
		if err != nil || !ok {
			writeJSON(w, http.StatusBadRequest, errorBody{
				Error: fmt.Sprintf("ip %s not in service_subnet %s",
					ip, s.deps.Config.WireGuard.ServiceSubnet)})
			return
		}
	}

	ports, err := parsePorts(req.Ports)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}

	if spec.Kind == origin.Host && len(ports) == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{
			Error: "a 'host' service must declare explicit ports (a wildcard host service would expose all loopback ports)"})
		return
	}

	sv, err := s.deps.Store.CreateService(r.Context(), store.CreateServiceInput{
		Name: req.Name, Hostname: hostname, Origin: spec, IPAddress: ip,
		Description: req.Description, Ports: ports,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toServiceDTO(sv))
}

func (s *Server) getService(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	sv, err := s.deps.Store.GetService(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toServiceDTO(sv))
}

type updateServiceReq = wire.UpdateServiceReq

func (s *Server) updateService(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req updateServiceReq
	if !readJSON(w, r, &req) {
		return
	}

	in := store.UpdateServiceInput{
		Name: req.Name, Hostname: req.Hostname,
		IPAddress: req.IP, Description: req.Description,
	}
	if req.Ports != nil {
		ports, err := parsePorts(*req.Ports)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
			return
		}
		in.Ports = ports
		in.ReplacePorts = true
	}

	originChanged := req.OriginKind != nil || req.OriginValue != nil || req.OriginNetwork != nil
	if originChanged || req.Ports != nil {
		cur, err := s.deps.Store.GetService(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}

		effOrigin := cur.Origin
		if originChanged {
			if req.OriginKind != nil {
				newKind := origin.Kind(*req.OriginKind)
				if newKind != effOrigin.Kind {
					effOrigin = origin.Spec{Kind: newKind}
				}
			}
			if req.OriginValue != nil {
				effOrigin.Value = *req.OriginValue
			}
			if req.OriginNetwork != nil {
				effOrigin.Network = *req.OriginNetwork
			}
			spec, verr := origin.Validate(effOrigin)
			if verr != nil {
				writeJSON(w, http.StatusBadRequest, errorBody{Error: verr.Error()})
				return
			}
			effOrigin = spec
			in.Origin = &effOrigin
		}

		effPorts := cur.Ports
		if in.ReplacePorts {
			effPorts = in.Ports
		}
		if effOrigin.Kind == origin.Host && len(effPorts) == 0 {
			writeJSON(w, http.StatusBadRequest, errorBody{
				Error: "a 'host' service must declare explicit ports (a wildcard host service would expose all loopback ports)"})
			return
		}
	}

	sv, err := s.deps.Store.UpdateService(r.Context(), id, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toServiceDTO(sv))
}

func (s *Server) deleteService(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := s.deps.Store.DeleteService(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) checkServiceOrigin(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	sv, err := s.deps.Store.GetService(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if s.deps.OriginChecker == nil {
		writeJSON(w, http.StatusOK, wire.OriginCheck{
			Detail: "origin checking is unavailable on this daemon",
		})
		return
	}

	port := 0
	for _, p := range sv.Ports {
		if p.Protocol == "tcp" && (port == 0 || p.Port < port) {
			port = p.Port
		}
	}

	target, err := s.deps.OriginChecker.Resolve(r.Context(), sv.Origin, port, "tcp")
	if err != nil {
		writeJSON(w, http.StatusOK, wire.OriginCheck{Reachable: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wire.OriginCheck{
		Reachable: true,
		DialHost:  target.DialHost,
		Network:   target.Network,
		Detail:    target.Detail,
	})
}

func parsePorts(spec string) ([]store.ServicePort, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var out []store.ServicePort
	for raw := range strings.SplitSeq(spec, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var portStr, proto string
		if before, after, ok := strings.Cut(raw, "/"); ok {
			portStr, proto = before, strings.ToLower(after)
		} else {
			portStr = raw
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("invalid port %q", raw)
		}
		switch proto {
		case "tcp":
			out = append(out, store.ServicePort{Port: port, Protocol: "tcp"})
		case "udp":
			out = append(out, store.ServicePort{Port: port, Protocol: "udp"})
		case "":
			out = append(out,
				store.ServicePort{Port: port, Protocol: "tcp"},
				store.ServicePort{Port: port, Protocol: "udp"})
		default:
			return nil, fmt.Errorf("invalid protocol %q in %q", proto, raw)
		}
	}
	return out, nil
}
