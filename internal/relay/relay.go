// Package relay implements a local WebRTC hub for `mugon dev`.
//
// The parentframe script connects to it directly over WebRTC (one connection
// per browser tab/instance) instead of establishing real peer-to-peer
// connections between browser tabs. Each connecting player is assigned a uuid
// and can address other connected players by that uuid; the relay forwards
// data frames between them. The "server" role player additionally receives
// connect/disconnect notifications for every other player, mirroring the
// client-server topology used by mugon games.
//
// Every player opens one data channel per delivery guarantee (reliable/unreliable
// x ordered/unordered). The relay pairs channels by index: a frame received on
// a player's channel i is forwarded on the target's channel i, so the chosen
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

// maxNameLength is the longest display name the platform accepts.
const maxNameLength = 32

// A frame is [type][player id][payload]. The payload of a framePlayerConnected
// is the new player's display name; for frameData it is the data.
type frameType byte

const (
	frameData               frameType = 0x01
	framePlayerConnected    frameType = 0x02
	framePlayerDisconnected frameType = 0x03
	frameDisconnectPlayer   frameType = 0x04
)

// channelLabels indexes the data channels by their delivery guarantee. The
// index is the Channel value shared with the SDK and parentframe; the label
// pairs a player's channel with its counterpart on the other player.
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
		return 0, uuid.UUID{}, nil, fmt.Errorf("relay: invalid player id in frame: %w", err)
	}
	return frameType(data[0]), id, data[1+uuidLen:], nil
}

type channel struct {
	dc      *webrtc.DataChannel
	ready   bool
	pending [][]byte
}

type player struct {
	id   uuid.UUID
	name string
	role role
	pc   *webrtc.PeerConnection
	// settings are the values the server player started the game with, which
	// clients fetch from /settings before they launch.
	settings json.RawMessage
	// link simulates the network conditions of a client; nil for the server.
	link *link

	mu       sync.Mutex
	channels [channelCount]channel
}

func (p *player) setDataChannel(index int, dc *webrtc.DataChannel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channels[index].dc = dc
}

func (p *player) markReady(index int) {
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
func (p *player) send(index int, frame []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := &p.channels[index]
	if !c.ready || c.dc == nil {
		c.pending = append(c.pending, frame)
		return
	}
	_ = c.dc.Send(frame)
}

// Relay is a WebRTC hub: every player connects directly to the relay (not to
// each other) and addresses other players by the uuid the relay assigned them.
type Relay struct {
	mu           sync.Mutex
	joined       int
	players      map[uuid.UUID]*player
	serverPlayer *player
	// profile is the simulated network condition of every client link. It is
	// reset whenever the server player changes.
	profile Profile
}

func New() *Relay {
	return &Relay{players: make(map[uuid.UUID]*player)}
}

func (r *Relay) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/offer", r.handleOffer)
	mux.HandleFunc("/settings", r.handleSettings)
	mux.HandleFunc("/netsim", r.handleNetsim)
	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
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
	// Name is the player's display name; empty gets a generated one.
	Name string `json:"name,omitempty"`
	// Settings are only read from a server; a client's are ignored.
	Settings json.RawMessage `json:"settings,omitempty"`
}

type offerResponse struct {
	SDP            string `json:"sdp"`
	PlayerID       string `json:"playerId"`
	Name           string `json:"name"`
	ServerPlayerID string `json:"serverPlayerId,omitempty"`
	ServerName     string `json:"serverName,omitempty"`
}

type settingsResponse struct {
	Settings json.RawMessage `json:"settings,omitempty"`
}

// handleSettings returns the settings the current server started the game with,
// or 404 while no server is connected.
func (r *Relay) handleSettings(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.mu.Lock()
	server := r.serverPlayer
	r.mu.Unlock()
	if server == nil {
		http.Error(w, "no server connected", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settingsResponse{Settings: server.settings})
}

type netsimRequest struct {
	// PlayerID must be the connected server's id: only the host tunes the network.
	PlayerID string  `json:"playerId"`
	Profile  Profile `json:"profile"`
}

type netsimResponse struct {
	Profile Profile `json:"profile"`
}

// handleNetsim reads (GET) or sets (PUT) the simulated network condition of
// every client link.
func (r *Relay) handleNetsim(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.mu.Lock()
		profile := r.profile
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(netsimResponse{Profile: profile})
	case http.MethodPut:
		var body netsimRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := body.Profile.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		r.mu.Lock()
		defer r.mu.Unlock()
		if r.serverPlayer == nil {
			http.Error(w, "no server connected", http.StatusNotFound)
			return
		}
		if r.serverPlayer.id.String() != body.PlayerID {
			http.Error(w, "only the server may change the network simulation", http.StatusForbidden)
			return
		}
		r.setProfileLocked(body.Profile)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Relay) setProfileLocked(profile Profile) {
	r.profile = profile
	for _, p := range r.players {
		if p.link != nil {
			p.link.setProfile(profile)
		}
	}
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

	playerRole := role(body.Role)
	if playerRole != roleServer && playerRole != roleClient {
		http.Error(w, "role must be 'server' or 'client'", http.StatusBadRequest)
		return
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create peer connection: %v", err), http.StatusInternalServerError)
		return
	}

	p := &player{id: uuid.New(), name: r.nameFor(body.Name), role: playerRole, pc: pc}
	if playerRole == roleServer {
		p.settings = body.Settings
	} else {
		r.mu.Lock()
		p.link = newLink(r.profile)
		r.mu.Unlock()
	}

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
			r.removePlayer(p)
		case webrtc.PeerConnectionStateUnknown, webrtc.PeerConnectionStateNew, webrtc.PeerConnectionStateConnecting, webrtc.PeerConnectionStateConnected:
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
		PlayerID: p.id.String(),
		Name:     p.name,
	}

	r.mu.Lock()
	r.players[p.id] = p
	if playerRole == roleServer {
		previousServer := r.serverPlayer
		r.serverPlayer = p
		r.setProfileLocked(Profile{})
		r.mu.Unlock()
		if previousServer != nil {
			go func() { _ = previousServer.pc.Close() }()
		}
	} else {
		if r.serverPlayer != nil {
			resp.ServerPlayerID = r.serverPlayer.id.String()
			resp.ServerName = r.serverPlayer.name
		}
		r.mu.Unlock()
	}

	if playerRole == roleClient {
		r.notifyServer(framePlayerConnected, p.id, []byte(p.name))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (r *Relay) handleMessage(sender *player, index int, data []byte) {
	t, targetID, payload, err := decodeFrame(data)
	if err != nil {
		return
	}

	switch t {
	case frameData:
		if target, ok := r.lookupPlayer(targetID); ok && mayAddress(sender, target) {
			deliver := func() { target.send(index, encodeFrame(frameData, sender.id, payload)) }
			if clientLink := linkBetween(sender, target); clientLink != nil {
				clientLink.submit(index, deliver)
			} else {
				deliver()
			}
		}
	case frameDisconnectPlayer:
		if target, ok := r.lookupPlayer(targetID); ok && mayAddress(sender, target) {
			go func() { _ = target.pc.Close() }()
		}
	case framePlayerConnected, framePlayerDisconnected:
	}
}

// linkBetween returns the client's link of a frame between the two players, or
// nil when neither is a client.
func linkBetween(sender, target *player) *link {
	if sender.link != nil {
		return sender.link
	}
	return target.link
}

// mayAddress reports whether sender is allowed to send a frame targeting
// target. Clients may only address the server; the server may address any
// connected player.
func mayAddress(sender, target *player) bool {
	if sender.role == roleServer {
		return true
	}
	return target.role == roleServer
}

// nameFor trims a requested display name to the platform's limit and numbers
// the players that did not bring one in the order they joined.
func (r *Relay) nameFor(requested string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.joined++
	if requested == "" {
		return fmt.Sprintf("Player %d", r.joined)
	}
	if runes := []rune(requested); len(runes) > maxNameLength {
		return string(runes[:maxNameLength])
	}
	return requested
}

func (r *Relay) lookupPlayer(id uuid.UUID) (*player, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[id]
	return p, ok
}

func (r *Relay) removePlayer(p *player) {
	r.mu.Lock()
	existing, ok := r.players[p.id]
	if !ok || existing != p {
		r.mu.Unlock()
		return
	}
	delete(r.players, p.id)
	wasServer := r.serverPlayer == p
	if wasServer {
		r.serverPlayer = nil
		r.setProfileLocked(Profile{})
	}
	r.mu.Unlock()

	if p.link != nil {
		p.link.close()
	}

	if !wasServer {
		r.notifyServer(framePlayerDisconnected, p.id, nil)
	}
}

func (r *Relay) notifyServer(t frameType, playerID uuid.UUID, payload []byte) {
	r.mu.Lock()
	server := r.serverPlayer
	r.mu.Unlock()
	if server == nil {
		return
	}
	server.send(channelReliableOrdered, encodeFrame(t, playerID, payload))
}
