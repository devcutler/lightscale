package api

import (
	"net/http"

	"github.com/devcutler/lightscale/shared/wire"
)

type peerDTO = wire.Peer

func (s *Server) listPeers(w http.ResponseWriter, r *http.Request) {
	statuses, err := s.deps.Peers.PeerStatus()
	if err != nil {
		writeError(w, err)
		return
	}
	userByPubKey := map[string]UserBrief{}
	if s.deps.Resolver != nil {
		for _, u := range s.deps.Resolver.Users() {
			userByPubKey[u.PublicKey] = u
		}
	}

	now := s.deps.Now()
	out := make([]peerDTO, 0, len(statuses))
	for _, p := range statuses {
		dto := peerDTO{
			PublicKey:         p.PublicKey,
			PresharedKey:      p.PresharedKey,
			AllowedIPs:        p.AllowedIPs,
			Endpoint:          p.Endpoint,
			LastHandshake:     p.LastHandshake,
			KeepaliveInterval: p.KeepaliveInterval,
			RxBytes:           p.RxBytes,
			TxBytes:           p.TxBytes,
		}
		if !p.LastHandshake.IsZero() {
			dto.LastHandshakeAgoS = int64(now.Sub(p.LastHandshake).Seconds())
		}
		if u, ok := userByPubKey[p.PublicKey]; ok {
			dto.UserID = u.ID
			dto.Name = u.Name
			dto.IPAddress = u.IP
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

type connectionDTO = wire.Connection

func (s *Server) listConnections(w http.ResponseWriter, r *http.Request) {
	flows := s.deps.Flows.Snapshot()
	res := s.deps.Resolver

	out := make([]connectionDTO, 0, len(flows))
	for _, f := range flows {
		dto := connectionDTO{
			ID:         f.ID,
			SrcUserID:  f.SrcUserID,
			ObjectType: f.ObjectType,
			ObjectID:   f.ObjectID,
			Port:       f.Port,
			Protocol:   f.Protocol,
		}
		if res != nil {
			if u, ok := res.UserByID(f.SrcUserID); ok {
				dto.SrcName = u.Name
				dto.SrcIP = u.IP
			}
			switch f.ObjectType {
			case "service":
				if sv, ok := res.ServiceByID(f.ObjectID); ok {
					dto.ObjectName = sv.Name
					dto.ObjectIP = sv.IP
				}
			case "user":
				if u, ok := res.UserByID(f.ObjectID); ok {
					dto.ObjectName = u.Name
					dto.ObjectIP = u.IP
				}
			}
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}
