package api

import (
	"fmt"
	"strings"

	"github.com/devcutler/lightscale/daemon/store"
)

func renderClientConf(u store.User, serverPubKey, endpoint, fullSubnet string) string {
	addrPrefix := strings.SplitN(fullSubnet, "/", 2)
	mask := "23"
	if len(addrPrefix) == 2 {
		mask = addrPrefix[1]
	}
	c := &wgConf{}
	c.section("Interface")
	c.add("PrivateKey", u.PrivateKey)
	c.add("Address", u.IPAddress+"/"+mask)
	// previously hardcoded for personal use, commented for publish
	// c.add("DNS", "1.1.1.1, 8.8.8.8")
	c.section("Peer")
	c.add("PublicKey", serverPubKey)
	c.add("PresharedKey", u.PresharedKey)
	c.add("AllowedIPs", fullSubnet)
	c.add("Endpoint", endpoint)
	c.add("PersistentKeepalive", "25")
	return c.String()
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
	c.lines = append(c.lines, fmt.Sprintf("%s = %s", key, value))
}

func (c *wgConf) String() string {
	return strings.Join(c.lines, "\n") + "\n"
}
