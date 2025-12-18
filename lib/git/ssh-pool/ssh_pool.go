package ssh_pool

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/kluctl/kluctl/lib/git/auth"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

const defaultMaxAge = time.Minute * 1
const defailtMaxErrorAge = time.Second * 10

type SshPool struct {
	MaxAge      time.Duration
	MaxErrorAge time.Duration

	pool map[string]*poolEntry

	m sync.Mutex
}

type poolEntry struct {
	hash string
	addr string

	sessionCount int
	time         time.Time

	client *ssh.Client
	err    error

	m sync.Mutex
}

type PooledSession struct {
	Session *ssh.Session
	pe      *poolEntry
}

func (p *SshPool) GetSession(ctx context.Context, host string, port int, auth auth.AuthMethodAndCA) (*PooledSession, error) {
	pe, isNew, err := p.getLockedPoolEntry(ctx, host, port, auth)
	if err != nil {
		return nil, err
	}
	defer pe.m.Unlock()

	if pe.err != nil {
		return nil, pe.err
	}

	s, err := pe.client.NewSession()
	if err != nil {
		_ = pe.client.Close()
		if isNew {
			// don't retry with a fresh connection and cause the pool entry to expire earlier
			pe.err = err
			return nil, err
		} else {
			// retry with a fresh connection
			client, err := p.newClient(ctx, pe.addr, auth)
			if err != nil {
				// make the pool entry expire earlier and return errors until then
				pe.err = err
				return nil, err
			}

			pe.client = client
			s, err = pe.client.NewSession()
			if err != nil {
				// something is completely wrong here
				pe.err = err
				return nil, err
			}
		}
	}

	pe.sessionCount += 1

	ps := &PooledSession{
		pe:      pe,
		Session: s,
	}
	return ps, nil
}

func (p *SshPool) init() {
	p.pool = map[string]*poolEntry{}
	if p.MaxAge == 0 {
		p.MaxAge = defaultMaxAge
	}
	if p.MaxErrorAge == 0 {
		p.MaxErrorAge = defailtMaxErrorAge
	}
}

func (p *SshPool) getLockedPoolEntry(ctx context.Context, host string, port int, auth auth.AuthMethodAndCA) (*poolEntry, bool, error) {
	addr := getHostWithPort(host, port)

	h, err := p.buildHash(addr, auth)
	if err != nil {
		return nil, false, err
	}

	isNew := false

	p.m.Lock()
	if p.pool == nil {
		p.init()
	}
	pe := p.pool[h]
	if pe == nil {
		isNew = true
		pe = &poolEntry{
			hash: h,
			addr: addr,
			time: time.Now(),
		}
		p.pool[h] = pe
	}
	p.m.Unlock()

	// caller must unlock
	pe.m.Lock()

	if pe.err != nil {
		return pe, isNew, nil
	}

	if isNew {
		client, err := p.newClient(ctx, addr, auth)
		if err != nil {
			pe.err = err
		} else {
			pe.client = client
		}
		p.startExpire(pe)
	}
	return pe, isNew, nil
}

func (p *SshPool) startExpire(pe *poolEntry) {
	go func() {
		for {
			time.Sleep(1 * time.Second)

			pe.m.Lock()
			if pe.sessionCount != 0 {
				pe.m.Unlock()
				continue
			}
			elapsed := time.Now().Sub(pe.time)
			expired := false
			if pe.err != nil && elapsed > p.MaxErrorAge {
				expired = true
			} else if pe.err == nil && elapsed >= p.MaxAge {
				expired = true
			}
			pe.m.Unlock()

			if expired {
				break
			}
		}

		p.m.Lock()
		delete(p.pool, pe.hash)
		p.m.Unlock()

		pe.m.Lock()
		defer pe.m.Unlock()
		if pe.client != nil {
			_ = pe.client.Close()
		}
		pe.client = nil
	}()
}

func (ps *PooledSession) Close() error {
	err := ps.Session.Close()

	ps.pe.m.Lock()
	defer ps.pe.m.Unlock()

	ps.pe.sessionCount--

	return err
}

func (p *SshPool) newClient(ctx context.Context, addr string, auth auth.AuthMethodAndCA) (*ssh.Client, error) {
	conn, err := proxy.Dial(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	config, err := auth.SshClientConfig()
	if err != nil {
		return nil, err
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func (p *SshPool) buildHash(addr string, auth auth.AuthMethodAndCA) (string, error) {
	if auth.Hash == nil {
		// this should not happen
		return "", fmt.Errorf("auth has no Hash")
	}

	config, err := auth.SshClientConfig()
	if err != nil {
		return "", err
	}

	h := sha256.New()
	_ = binary.Write(h, binary.LittleEndian, addr)
	_ = binary.Write(h, binary.LittleEndian, config.User)

	h2, err := auth.Hash()
	if err != nil {
		return "", err
	}
	_ = binary.Write(h, binary.LittleEndian, h2)

	return hex.EncodeToString(h.Sum(nil)), nil
}
