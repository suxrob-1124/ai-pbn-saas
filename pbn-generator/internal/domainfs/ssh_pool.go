package domainfs

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHPoolConfig struct {
	MaxOpen        int
	MaxIdle        int
	IdleTTL        time.Duration
	DialTimeout    time.Duration
	KnownHostsPath string
}

type SSHPool struct {
	cfg SSHPoolConfig

	hostKeyCallback ssh.HostKeyCallback

	mu      sync.Mutex
	closed  bool
	buckets map[string]*sshPoolBucket
	signers map[string]ssh.Signer
}

type sshPoolBucket struct {
	mu     sync.Mutex
	closed bool
	open   int
	idle   []idleSSHClient
	notify chan struct{}
}

type idleSSHClient struct {
	client   *ssh.Client
	released time.Time
}

type SSHClientLease struct {
	Client *ssh.Client

	pool   *SSHPool
	bucket *sshPoolBucket
	once   sync.Once
}

func NewSSHPool(cfg SSHPoolConfig) (*SSHPool, error) {
	if cfg.MaxOpen <= 0 {
		return nil, fmt.Errorf("ssh pool max open must be > 0")
	}
	if cfg.MaxIdle < 0 {
		return nil, fmt.Errorf("ssh pool max idle must be >= 0")
	}
	if cfg.MaxIdle > cfg.MaxOpen {
		return nil, fmt.Errorf("ssh pool max idle must be <= max open")
	}
	if cfg.IdleTTL <= 0 {
		return nil, fmt.Errorf("ssh pool idle ttl must be > 0")
	}
	if cfg.DialTimeout <= 0 {
		return nil, fmt.Errorf("ssh pool dial timeout must be > 0")
	}
	knownHostsPath := strings.TrimSpace(cfg.KnownHostsPath)
	if knownHostsPath == "" {
		return nil, fmt.Errorf("ssh pool known_hosts path is required")
	}
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("known_hosts callback init failed: %w", err)
	}
	return &SSHPool{
		cfg:             cfg,
		hostKeyCallback: callback,
		buckets:         make(map[string]*sshPoolBucket),
		signers:         make(map[string]ssh.Signer),
	}, nil
}

func (p *SSHPool) Acquire(ctx context.Context, target SSHTarget) (*SSHClientLease, error) {
	if err := validateSSHTarget(target); err != nil {
		return nil, err
	}
	bucket, err := p.bucketFor(target)
	if err != nil {
		return nil, err
	}
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if client := p.tryAcquireIdle(bucket); client != nil {
			return &SSHClientLease{
				Client: client,
				pool:   p,
				bucket: bucket,
			}, nil
		}
		if p.tryReserveOpen(bucket) {
			client, err := p.dial(target)
			if err != nil {
				p.releaseReservedOpen(bucket)
				return nil, err
			}
			return &SSHClientLease{
				Client: client,
				pool:   p,
				bucket: bucket,
			}, nil
		}
		if err := p.waitForAvailability(ctx, bucket); err != nil {
			return nil, err
		}
	}
}

func (l *SSHClientLease) Release() {
	l.once.Do(func() {
		if l.pool == nil || l.bucket == nil || l.Client == nil {
			return
		}
		l.pool.release(l.bucket, l.Client, false)
	})
}

func (l *SSHClientLease) Discard() {
	l.once.Do(func() {
		if l.pool == nil || l.bucket == nil || l.Client == nil {
			return
		}
		l.pool.release(l.bucket, l.Client, true)
	})
}

func (p *SSHPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	buckets := make([]*sshPoolBucket, 0, len(p.buckets))
	for _, bucket := range p.buckets {
		buckets = append(buckets, bucket)
	}
	p.mu.Unlock()

	for _, bucket := range buckets {
		bucket.mu.Lock()
		bucket.closed = true
		for _, item := range bucket.idle {
			_ = item.client.Close()
		}
		bucket.open -= len(bucket.idle)
		bucket.idle = nil
		notifyChan(bucket.notify)
		bucket.mu.Unlock()
	}
	return nil
}

func (p *SSHPool) bucketFor(target SSHTarget) (*sshPoolBucket, error) {
	key := strings.TrimSpace(target.Alias)
	if key == "" {
		key = strings.Join([]string{target.Host, target.User, target.KeyPath, fmt.Sprintf("%d", target.Port)}, "|")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, fmt.Errorf("ssh pool is closed")
	}
	if bucket, ok := p.buckets[key]; ok {
		return bucket, nil
	}
	bucket := &sshPoolBucket{
		notify: make(chan struct{}, 1),
	}
	p.buckets[key] = bucket
	return bucket, nil
}

func (p *SSHPool) tryAcquireIdle(bucket *sshPoolBucket) *ssh.Client {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	if bucket.closed {
		return nil
	}
	p.pruneIdleLocked(bucket)
	if n := len(bucket.idle); n > 0 {
		item := bucket.idle[n-1]
		bucket.idle = bucket.idle[:n-1]
		return item.client
	}
	return nil
}

func (p *SSHPool) tryReserveOpen(bucket *sshPoolBucket) bool {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	if bucket.closed {
		return false
	}
	if bucket.open >= p.cfg.MaxOpen {
		return false
	}
	bucket.open++
	return true
}

func (p *SSHPool) releaseReservedOpen(bucket *sshPoolBucket) {
	bucket.mu.Lock()
	if bucket.open > 0 {
		bucket.open--
	}
	notifyChan(bucket.notify)
	bucket.mu.Unlock()
}

func (p *SSHPool) waitForAvailability(ctx context.Context, bucket *sshPoolBucket) error {
	bucket.mu.Lock()
	notify := bucket.notify
	bucket.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-notify:
		return nil
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (p *SSHPool) release(bucket *sshPoolBucket, client *ssh.Client, broken bool) {
	if client == nil {
		return
	}
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	shouldClose := broken || bucket.closed || p.isClosed()
	if shouldClose {
		if bucket.open > 0 {
			bucket.open--
		}
		_ = client.Close()
		notifyChan(bucket.notify)
		return
	}

	if p.cfg.MaxIdle == 0 || len(bucket.idle) >= p.cfg.MaxIdle {
		if bucket.open > 0 {
			bucket.open--
		}
		_ = client.Close()
		notifyChan(bucket.notify)
		return
	}

	bucket.idle = append(bucket.idle, idleSSHClient{
		client:   client,
		released: time.Now().UTC(),
	})
	notifyChan(bucket.notify)
}

func (p *SSHPool) pruneIdleLocked(bucket *sshPoolBucket) {
	if len(bucket.idle) == 0 {
		return
	}
	now := time.Now().UTC()
	alive := bucket.idle[:0]
	for _, item := range bucket.idle {
		if now.Sub(item.released) > p.cfg.IdleTTL {
			if bucket.open > 0 {
				bucket.open--
			}
			_ = item.client.Close()
			continue
		}
		alive = append(alive, item)
	}
	bucket.idle = alive
}

func (p *SSHPool) dial(target SSHTarget) (*ssh.Client, error) {
	signer, err := p.signerFor(target.KeyPath)
	if err != nil {
		return nil, err
	}
	clientCfg := &ssh.ClientConfig{
		User:            strings.TrimSpace(target.User),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: p.hostKeyCallback,
		Timeout:         p.cfg.DialTimeout,
	}
	addr := target.Address()
	conn, err := net.DialTimeout("tcp", addr, p.cfg.DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("ssh dial failed (%s): %w", addr, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, clientCfg)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh handshake failed (%s): %w", addr, err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func (p *SSHPool) signerFor(keyPath string) (ssh.Signer, error) {
	keyPath = strings.TrimSpace(keyPath)
	if keyPath == "" {
		return nil, fmt.Errorf("ssh key path is empty")
	}
	p.mu.Lock()
	if signer, ok := p.signers[keyPath]; ok {
		p.mu.Unlock()
		return signer, nil
	}
	p.mu.Unlock()

	raw, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh key %s: %w", keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse ssh key %s: %w", keyPath, err)
	}

	p.mu.Lock()
	if existing, ok := p.signers[keyPath]; ok {
		p.mu.Unlock()
		return existing, nil
	}
	p.signers[keyPath] = signer
	p.mu.Unlock()
	return signer, nil
}

func (p *SSHPool) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func validateSSHTarget(target SSHTarget) error {
	if strings.TrimSpace(target.Host) == "" {
		return fmt.Errorf("ssh target host is required")
	}
	if strings.TrimSpace(target.User) == "" {
		return fmt.Errorf("ssh target user is required")
	}
	if strings.TrimSpace(target.KeyPath) == "" {
		return fmt.Errorf("ssh target key path is required")
	}
	return nil
}

func notifyChan(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
