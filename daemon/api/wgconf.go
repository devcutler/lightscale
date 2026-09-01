package api

import (
	"fmt"
	"strings"

	"github.com/devcutler/lightscale/daemon/store"
)

func renderClientConf(u store.User, serverPubKey, endpoint, fullSubnet string, dns []string) string {
	addrPrefix := strings.SplitN(fullSubnet, "/", 2)
	mask := "23"
	if len(addrPrefix) == 2 {
		mask = addrPrefix[1]
	}
	c := &wgConf{}
	c.section("Interface")
	c.add("PrivateKey", u.PrivateKey)
	c.add("Address", u.IPAddress+"/"+mask)
	c.add("DNS", joinDNS(dns))
	c.section("Peer")
	c.add("PublicKey", serverPubKey)
	c.add("PresharedKey", u.PresharedKey)
	c.add("AllowedIPs", fullSubnet)
	c.add("Endpoint", endpoint)
	c.add("PersistentKeepalive", "25")
	return c.String()
}

func joinDNS(dns []string) string {
	out := make([]string, 0, len(dns))
	for _, s := range dns {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}

type wgConf struct {
	lines []string
}

func (c *wgConf) section(name string) {
	if len(c.lines) > 0 {
		c.lines = append(c.lines, "")
	}
	c.lines = append(c.lines, fmt.Sprintf("[%s]", name))
}

func (c *wgConf) add(key, value string) {
	if value == "" {
		return
	}
	c.lines = append(c.lines, fmt.Sprintf("%s = %s", key, value))
}

func (c *wgConf) String() string {
	return strings.Join(c.lines, "\n") + "\n"
}
