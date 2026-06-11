package daemon

import (
	"fmt"
	"net"
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/devcutler/lightscale/shared/config"
)

type peerCredListener struct {
	net.Listener
	selfUID    uint32
	allowedGID uint32
	hasGID     bool
}

func newPeerCredListener(ln net.Listener, cfg config.SocketConfig) (net.Listener, error) {
	l := &peerCredListener{
		Listener: ln,
		selfUID:  uint32(unix.Getuid()),
	}
	if cfg.Group != "" {
		grp, err := user.LookupGroup(cfg.Group)
		if err != nil {
			return nil, fmt.Errorf("peercred: lookup group %q: %w", cfg.Group, err)
		}
		gid, err := strconv.ParseUint(grp.Gid, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("peercred: parse gid %q: %w", grp.Gid, err)
		}
		l.allowedGID = uint32(gid)
		l.hasGID = true
	}
	return l, nil
}

func (l *peerCredListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		ok, cerr := l.authorized(c)
		if cerr != nil {
			_ = c.Close()
			continue
		}
		if !ok {
			_ = c.Close()
			continue
		}
		return c, nil
	}
}
func (l *peerCredListener) authorized(c net.Conn) (bool, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return false, fmt.Errorf("peercred: not a unix conn (%T)", c)
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return false, err
	}
	var cred *unix.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return false, err
	}
	if credErr != nil {
		return false, credErr
	}
	if cred.Uid == l.selfUID {
		return true, nil
	}
	if l.hasGID {
		if cred.Gid == l.allowedGID {
			return true, nil
		}
		// SO_PEERCRED is only primary, resolve for secondary
		if l.inAllowedGroup(cred.Uid) {
			return true, nil
		}
	}
	return false, nil
}

func (l *peerCredListener) inAllowedGroup(uid uint32) bool {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return false
	}
	gids, err := u.GroupIds()
	if err != nil {
		return false
	}
	want := strconv.FormatUint(uint64(l.allowedGID), 10)
	for _, g := range gids {
		if g == want {
			return true
		}
	}
	return false
}
