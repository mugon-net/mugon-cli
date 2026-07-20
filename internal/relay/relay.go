// Package relay implements a local WebRTC hub for `mugon dev`.
//
// The parentframe script connects to it directly over WebRTC (one connection
// per browser tab/instance) instead of establishing real peer-to-peer
// connections between browser tabs. Each connecting peer is assigned a uuid
// and can address other connected peers by that uuid; the relay forwards
// data frames between them. The "server" role peer additionally receives
// connect/disconnect notifications for every other peer, mirroring the
// client-server topology used by mugon games.
//
// Every peer opens one data channel per delivery guarantee (reliable/unreliable
// x ordered/unordered). The relay pairs channels by index: a frame received on
// a peer's channel i is forwarded on the target's channel i, so the chosen
// delivery guarantee is preserved across both hops.
package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type role string

const (
	roleServer role = "server"
	roleClient role = "client"
)

type frameType byte

const (
	frameData         frameType = 0x01
	frameConnected    frameType = 0x02
	frameDisconnected frameType = 0x03
	frameDrop         frameType = 0x04
)

// channelLabels indexes the data channels by their delivery guarantee. The
// index is the Channel value shared with the SDK and parentframe; the label
// pairs a peer's channel with its counterpart on the other peer.
var channelLabels = [...]string{"ro", "ru", "uo", "uu"}

const channelCount = len(channelLabels)

// Control notifications (connect/disconnect) must always arrive, so they go on
// the reliable ordered channel.
const channelReliableOrdered = 0

func channelIndexForLabel(label string) (int, bool) {
	for i, l := range channelLabels {
		if l == label {
			return i, true
		}
	}
	return 0, false
}

// uuid.String() always returns the 36-character canonical form.
const uuidLen = 36

func encodeFrame(t frameType, id uuid.UUID, payload []byte) []byte {
	frame := make([]byte, 1+uuidLen+len(payload))
	frame[0] = byte(t)
	copy(frame[1:], id.String())
	copy(frame[1+uuidLen:], payload)
	return frame
}

func decodeFrame(data []byte) (frameType, uuid.UUID, []byte, error) {
	if len(data) < 1+uuidLen {
		return 0, uuid.UUID{}, nil, fmt.Errorf("relay: frame too short")
	}
	id, err := uuid.Parse(string(data[1 : 1+uuidLen]))
	if err != nil {
		return 0, uuid.UUID{}, nil, fmt.Errorf("relay: invalid peer id in frame: %w", err)
	}
	return frameType(data[0]), id, data[1+uuidLen:], nil
}

type channel struct {
	dc      *webrtc.DataChannel
	ready   bool
	pending [][]byte
}

type peer struct {
	id   uuid.UUID
	role role
	pc   *webrtc.PeerConnection

	mu       sync.Mutex
	channels [channelCount]channel
}

func (p *peer) setDataChannel(index int, dc *webrtc.DataChannel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channels[index].dc = dc
}

func (p *peer) markReady(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := &p.channels[index]
	c.ready = true
	for _, frame := range c.pending {
		_ = c.dc.Send(frame)
	}
	c.pending = nil
}

// send queues the frame until the channel opens if it is not ready yet, so
// notifications emitted during connection setup are not lost.
func (p *peer) send(index int, frame []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := &p.channels[index]
	if !c.ready || c.dc == nil {
		c.pending = append(c.pending, frame)
		return
	}
	_ = c.dc.Send(frame)
}

// Relay is a WebRTC hub: every peer connects directly to the relay (not to
// each other) and addresses other peers by the uuid the relay assigned them.
type Relay struct {
	mu         sync.Mutex
	peers      map[uuid.UUID]*peer
	serverPeer *peer
}

func New() *Relay {
	return &Relay{peers: make(map[uuid.UUID]*peer)}
}

func (r *Relay) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/offer", r.handleOffer)
	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

type offerRequest struct {
	SDP  string `json:"sdp"`
	Role string `json:"role"`
}

type offerResponse struct {
	SDP      string `json:"sdp"`
	ClientID string `json:"clientId"`
	ServerID string `json:"serverId,omitempty"`
}

func (r *Relay) handleOffer(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body offerRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	peerRole := role(body.Role)
	if peerRole != roleServer && peerRole != roleClient {
		http.Error(w, "role must be 'server' or 'client'", http.StatusBadRequest)
		return
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create peer connection: %v", err), http.StatusInternalServerError)
		return
	}

	p := &peer{id: uuid.New(), role: peerRole, pc: pc}

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		index, ok := channelIndexForLabel(dc.Label())
		if !ok {
			return
		}
		p.setDataChannel(index, dc)
		dc.OnOpen(func() { p.markReady(index) })
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			r.handleMessage(p, index, msg.Data)
		})
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		switch s {
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			r.removePeer(p)
		default:
		}
	})

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  body.SDP,
	}); err != nil {
		_ = pc.Close()
		http.Error(w, fmt.Sprintf("failed to set remote description: %v", err), http.StatusBadRequest)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		http.Error(w, fmt.Sprintf("failed to create answer: %v", err), http.StatusInternalServerError)
		return
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		http.Error(w, fmt.Sprintf("failed to set local description: %v", err), http.StatusInternalServerError)
		return
	}

	<-gatherComplete

	resp := offerResponse{
		SDP:      pc.LocalDescription().SDP,
		ClientID: p.id.String(),
	}

	r.mu.Lock()
	r.peers[p.id] = p
	if peerRole == roleServer {
		previousServer := r.serverPeer
		r.serverPeer = p
		r.mu.Unlock()
		if previousServer != nil {
			go func() { _ = previousServer.pc.Close() }()
		}
	} else {
		if r.serverPeer != nil {
			resp.ServerID = r.serverPeer.id.String()
		}
		r.mu.Unlock()
	}

	if peerRole == roleClient {
		r.notifyServer(frameConnected, p.id)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (r *Relay) handleMessage(sender *peer, index int, data []byte) {
	t, targetID, payload, err := decodeFrame(data)
	if err != nil {
		return
	}

	switch t {
	case frameData:
		if target, ok := r.lookupPeer(targetID); ok && mayAddress(sender, target) {
			target.send(index, encodeFrame(frameData, sender.id, payload))
		}
	case frameDrop:
		if target, ok := r.lookupPeer(targetID); ok && mayAddress(sender, target) {
			go func() { _ = target.pc.Close() }()
		}
	default:
	}
}

// mayAddress reports whether sender is allowed to send a frame targeting
// target. Clients may only address the server; the server may address any
// connected peer.
func mayAddress(sender, target *peer) bool {
	if sender.role == roleServer {
		return true
	}
	return target.role == roleServer
}

func (r *Relay) lookupPeer(id uuid.UUID) (*peer, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.peers[id]
	return p, ok
}

func (r *Relay) removePeer(p *peer) {
	r.mu.Lock()
	existing, ok := r.peers[p.id]
	if !ok || existing != p {
		r.mu.Unlock()
		return
	}
	delete(r.peers, p.id)
	wasServer := r.serverPeer == p
	if wasServer {
		r.serverPeer = nil
	}
	r.mu.Unlock()

	if !wasServer {
		r.notifyServer(frameDisconnected, p.id)
	}
}

func (r *Relay) notifyServer(t frameType, peerID uuid.UUID) {
	r.mu.Lock()
	server := r.serverPeer
	r.mu.Unlock()
	if server == nil {
		return
	}
	server.send(channelReliableOrdered, encodeFrame(t, peerID, nil))
}
