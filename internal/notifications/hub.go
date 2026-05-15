package notifications

import "sync"

type Hub struct {
	clients    map[string]map[*Client]bool
	mu         sync.RWMutex
	broadcast  chan BroadcastMessage
	register   chan *Client
	unregister chan *Client
}

type BroadcastMessage struct {
	BranchID string
	Data     []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan BroadcastMessage, 64),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) BroadcastToBranch(branchID string, message []byte) {
	h.broadcast <- BroadcastMessage{BranchID: branchID, Data: message}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.BranchID] == nil {
				h.clients[client.BranchID] = make(map[*Client]bool)
			}
			h.clients[client.BranchID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if branch, ok := h.clients[client.BranchID]; ok {
				if _, ok := branch[client]; ok {
					delete(branch, client)
					close(client.send)
					// FIX 3a (Branch cleanup): Already present — kept and confirmed correct.
					// Empty branch maps are removed so the clients map doesn't grow unboundedly
					// as branches come and go.
					if len(branch) == 0 {
						delete(h.clients, client.BranchID)
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			// FIX 3b (Unsafe RLock→Lock upgrade):
			// The original code held an RLock while iterating clients, then
			// attempted to acquire a write Lock inside the loop to close a slow
			// client's send channel. This is a deadlock: RLock and Lock are
			// mutually exclusive — the inner Lock call will block forever while
			// the outer RLock is still held on the same goroutine (sync.RWMutex
			// is not reentrant).
			//
			// Fix: collect slow clients during the read-locked pass, then
			// evict them all in a single subsequent write-locked pass.
			h.mu.RLock()
			branch := h.clients[msg.BranchID]
			var slow []*Client
			for client := range branch {
				select {
				case client.send <- msg.Data:
				default:
					// Buffer full — mark for eviction; do NOT lock here.
					slow = append(slow, client)
				}
			}
			h.mu.RUnlock()

			// Evict slow clients under a write lock only if there are any.
			if len(slow) > 0 {
				h.mu.Lock()
				for _, client := range slow {
					// FIX 3c: Re-check existence before closing; another goroutine
					// (e.g. the read loop calling Unregister) may have already
					// removed this client between the RUnlock and this Lock.
					if branch, ok := h.clients[msg.BranchID]; ok {
						if _, ok := branch[client]; ok {
							close(client.send)
							delete(branch, client)
							if len(branch) == 0 {
								delete(h.clients, msg.BranchID)
							}
						}
					}
				}
				h.mu.Unlock()
			}
		}
	}
}
