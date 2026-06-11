package api

import (
	"net/http"

	"github.com/devcutler/lightscale/daemon/dns"
)

func (s *Server) getDNSZone(w http.ResponseWriter, r *http.Request) {
	services, err := s.deps.Store.ListServices(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	zone := dns.Zone{
		Domain:  s.deps.Config.Domain,
		Now:     s.deps.Now(),
		Records: make([]dns.Record, 0, len(services)),
	}
	for _, sv := range services {
		zone.Records = append(zone.Records, dns.Record{
			Name: dns.LeafLabel(sv.Hostname),
			IP:   sv.IPAddress,
		})
	}

	body, err := dns.Render(zone)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}
