package ssh_pool

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
	"sync"
	"time"
)

const maxAge = time.Minute * 1

type SshPool struct {
	pool sync.Map
}

type poolEntry struct {
	hash string
	pc   *poolClient
	m    sync.Mutex
}

type poolClient struct {
	time         time.Time
	sessionCount int
	client       *ssh.Client
}

type PooledSession struct {
	Session *ssh.Session
	pe      *poolEntry
	pc      *poolClient
	hash    string
}

func (p *SshPool) GetSession(ctx context.Context, host string, port int, auth auth.AuthMethodAndCA) (*PooledSession, error) {
	addr := getHostWithPort(host, port)

	h, err := p.buildHash(addr, auth)
	if err != nil {
		return nil, err
	}

	pe1, _ := p.pool.LoadOrStore(h, &poolEntry{hash: h})
	pe, _ := pe1.(*poolEntry)

	pe.m.Lock()
	defer pe.m.Unlock()

	if pe.pc != nil {
		if time.Now().Sub(pe.pc.time) > maxAge {
			if pe.pc.sessionCount == 0 {
				_ = pe.pc.client.Close()
			}
			pe.pc = nil
		}
	}

	isNew := false
	if pe.pc == nil {
		isNew = true
		client, err := p.newClient(ctx, addr, auth)
		if err != nil {
			p.pool.Delete(h)
			return nil, err
		}
		pe.pc = &poolClient{
			time:   time.Now(),
			client: client,
		}
	}

	s, err := pe.pc.client.NewSession()
	if err != nil {
		origErr := err
		_ = pe.pc.client.Close()
		pe.pc = nil
		if isNew {
			// don't retry with a fresh connection
			p.pool.Delete(h)
			return nil, err
		} else {
			// retry with a fresh connection
			client, err := p.newClient(ctx, addr, auth)
			if err != nil {
				p.pool.Delete(h)
				return nil, err
			}
			status.Trace(ctx, "Successfully retries failed ssh connection. Old error: %s", origErr)
			pe.pc = &poolClient{
				time:   time.Now(),
				client: client,
			}
			s, err = pe.pc.client.NewSession()
			if err != nil {
				// something is completely wrong here, forget about the client
				p.pool.Delete(h)
				return nil, err
			}
		}
	}

	pe.pc.sessionCount += 1

	ps := &PooledSession{
		hash:    h,
		pe:      pe,
		pc:      pe.pc,
		Session: s,
	}
	return ps, nil
}

func (ps *PooledSession) Close() error {
	err := ps.Session.Close()

	ps.pe.m.Lock()
	defer ps.pe.m.Unlock()

	ps.pc.sessionCount--

	if ps.pe.pc != ps.pc {
		_ = ps.pc.client.Close()
	} else if ps.pc.sessionCount == 0 && time.Now().Sub(ps.pc.time) > maxAge {
		_ = ps.pc.client.Close()
		ps.pe.pc = nil
	}

	return err
}

func (p *SshPool) newClient(ctx context.Context, addr string, auth auth.AuthMethodAndCA) (*ssh.Client, error) {
	conn, err := proxy.Dial(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	config, err := auth.ClientConfig()
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

	config, err := auth.ClientConfig()
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
