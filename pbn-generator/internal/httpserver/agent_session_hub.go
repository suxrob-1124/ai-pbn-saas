package httpserver

import (
	"sync"

	"github.com/google/uuid"
)

// agentHubEvent is a single SSE event buffered in the hub.
type agentHubEvent struct {
	EventType string
	Payload   any
}

// hubSession is one active agent session in the event hub.
type hubSession struct {
	mu       sync.Mutex
	buf      []agentHubEvent
	subs     map[string]chan agentHubEvent
	done     bool
	liveDiag []byte // latest agentDiagnostics JSON; updated each iteration
}

// setLiveDiag stores a diagnostics snapshot for the active session.
func (hs *hubSession) setLiveDiag(b []byte) {
	hs.mu.Lock()
	hs.liveDiag = b
	hs.mu.Unlock()
}

// getLiveDiag returns the latest diagnostics snapshot, or nil if none yet.
func (hs *hubSession) getLiveDiag() []byte {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.liveDiag
}

// agentSessionHub manages in-memory state for running agent sessions.
type agentSessionHub struct {
	mu             sync.RWMutex
	sessions       map[string]*hubSession
	domainSessions map[string]string // domainID → active sessionID
	sem            chan struct{}      // global concurrency limit
}

const maxConcurrentAgentLoops = 10

var globalAgentHub = &agentSessionHub{
	sessions:       make(map[string]*hubSession),
	domainSessions: make(map[string]string),
	sem:            make(chan struct{}, maxConcurrentAgentLoops),
}

// registerForDomain atomically registers a hub session for the domain.
// Returns (nil, false) if the domain already has an active session.
func (h *agentSessionHub) registerForDomain(sessionID, domainID string) (*hubSession, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.domainSessions[domainID]; exists {
		return nil, false
	}
	hs := &hubSession{subs: make(map[string]chan agentHubEvent)}
	h.sessions[sessionID] = hs
	h.domainSessions[domainID] = sessionID
	return hs, true
}

func (h *agentSessionHub) lookup(sessionID string) (*hubSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	hs, ok := h.sessions[sessionID]
	return hs, ok
}

// getActiveDomainSession returns the session ID currently running for the domain.
func (h *agentSessionHub) getActiveDomainSession(domainID string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	id, ok := h.domainSessions[domainID]
	return id, ok
}

func (h *agentSessionHub) unregister(sessionID, domainID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, sessionID)
	if h.domainSessions[domainID] == sessionID {
		delete(h.domainSessions, domainID)
	}
}

// acquireSem tries to acquire a slot in the global concurrency semaphore.
// Returns false immediately if all slots are occupied.
func (h *agentSessionHub) acquireSem() bool {
	select {
	case h.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (h *agentSessionHub) releaseSem() {
	<-h.sem
}

// publish sends an event to all subscribers and appends it to the buffer.
func (hs *hubSession) publish(eventType string, payload any) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	ev := agentHubEvent{EventType: eventType, Payload: payload}
	hs.buf = append(hs.buf, ev)
	for _, ch := range hs.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// subscribe registers a new SSE consumer. Returns subID, event channel,
// and a snapshot of already-buffered events to replay.
func (hs *hubSession) subscribe() (string, chan agentHubEvent, []agentHubEvent) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	id := uuid.New().String()
	ch := make(chan agentHubEvent, 1024)
	hs.subs[id] = ch
	snap := make([]agentHubEvent, len(hs.buf))
	copy(snap, hs.buf)
	return id, ch, snap
}

func (hs *hubSession) unsubscribe(id string) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	delete(hs.subs, id)
}

// markDone closes all subscriber channels and marks the session as done.
func (hs *hubSession) markDone() {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.done = true
	for _, ch := range hs.subs {
		close(ch)
	}
	hs.subs = make(map[string]chan agentHubEvent)
}

func (hs *hubSession) isDone() bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.done
}
