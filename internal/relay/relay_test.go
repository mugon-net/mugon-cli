package relay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type testClient struct {
	pc       *webrtc.PeerConnection
	channels [channelCount]*webrtc.DataChannel
	messages [channelCount]chan []byte

	playerID       string
	name           string
	serverPlayerID string
	serverName     string
}

func mustParseID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("uuid.Parse(%q): %v", s, err)
	}
	return id
}

// channelInit mirrors the delivery guarantee the parentframe assigns to each
// channel index.
func channelInit(index int) *webrtc.DataChannelInit {
	ordered := index == 0 || index == 2
	init := &webrtc.DataChannelInit{Ordered: &ordered}
	if index >= 2 {
		maxRetransmits := uint16(0)
		init.MaxRetransmits = &maxRetransmits
	}
	return init
}

func dialRelay(t *testing.T, srvURL string, role string) *testClient {
	t.Helper()
	return dialRelayWithSettings(t, srvURL, role, nil)
}

func dialRelayWithSettings(t *testing.T, srvURL string, role string, settings json.RawMessage) *testClient {
	t.Helper()
	return dialRelayAs(t, srvURL, role, "", settings)
}

func dialRelayAs(t *testing.T, srvURL string, role string, name string, settings json.RawMessage) *testClient {
	t.Helper()

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	tc := &testClient{pc: pc}

	var wg sync.WaitGroup
	wg.Add(channelCount)
	for i := range channelCount {
		dc, err := pc.CreateDataChannel(channelLabels[i], channelInit(i))
		if err != nil {
			t.Fatalf("CreateDataChannel(%q): %v", channelLabels[i], err)
		}
		tc.channels[i] = dc
		tc.messages[i] = make(chan []byte, 16)
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			tc.messages[i] <- msg.Data
		})
		dc.OnOpen(func() { wg.Done() })
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}
	<-gatherComplete

	reqBody, _ := json.Marshal(offerRequest{SDP: pc.LocalDescription().SDP, Role: role, Name: name, Settings: settings})
	resp, err := http.Post(srvURL+"/offer", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST /offer: %v", err)
	}
	defer resp.Body.Close() // nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /offer: status %d", resp.StatusCode)
	}

	var body offerResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode offer response: %v", err)
	}

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: body.SDP}); err != nil {
		t.Fatalf("SetRemoteDescription: %v", err)
	}

	allOpen := make(chan struct{})
	go func() { wg.Wait(); close(allOpen) }()
	select {
	case <-allOpen:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for data channels to open")
	}

	tc.playerID = body.PlayerID
	tc.name = body.Name
	tc.serverPlayerID = body.ServerPlayerID
	tc.serverName = body.ServerName
	return tc
}

func (tc *testClient) recv(t *testing.T, index int, timeout time.Duration) []byte {
	t.Helper()
	select {
	case msg := <-tc.messages[index]:
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func TestRelayPlayerNames(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelayAs(t, srv.URL, "server", "Sam", nil)
	if server.name != "Sam" {
		t.Fatalf("server name = %q, want Sam", server.name)
	}

	named := dialRelayAs(t, srv.URL, "client", "Cleo", nil)
	if named.name != "Cleo" || named.serverName != "Sam" {
		t.Fatalf("named client got name %q and server name %q", named.name, named.serverName)
	}
	unnamed := dialRelay(t, srv.URL, "client")
	if unnamed.name != "Player 3" {
		t.Fatalf("unnamed client name = %q, want Player 3", unnamed.name)
	}
	long := dialRelayAs(t, srv.URL, "client", strings.Repeat("x", 40), nil)
	if len([]rune(long.name)) != maxNameLength {
		t.Fatalf("long name has %d characters, want %d", len([]rune(long.name)), maxNameLength)
	}

	for _, want := range []string{"Cleo", "Player 3", long.name} {
		_, _, payload, err := decodeFrame(server.recv(t, channelReliableOrdered, 5*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if string(payload) != want {
			t.Fatalf("connected frame carries %q, want %q", payload, want)
		}
	}
}

func TestRelayEndToEnd(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelay(t, srv.URL, "server")
	if server.playerID == "" {
		t.Fatal("server did not receive a playerId")
	}

	client := dialRelay(t, srv.URL, "client")
	if client.playerID == "" {
		t.Fatal("client did not receive a playerId")
	}
	if client.serverPlayerID != server.playerID {
		t.Fatalf("client.serverPlayerID = %q, want %q", client.serverPlayerID, server.playerID)
	}

	// Connect notifications arrive on the reliable ordered channel.
	connectedFrame := server.recv(t, channelReliableOrdered, 5*time.Second)
	ft, id, _, err := decodeFrame(connectedFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != framePlayerConnected || id.String() != client.playerID {
		t.Fatalf("expected connected frame for %s, got type=%d id=%s", client.playerID, ft, id)
	}

	payload := []byte("hello from client")
	if err := client.channels[channelReliableOrdered].Send(encodeFrame(frameData, mustParseID(t, client.serverPlayerID), payload)); err != nil {
		t.Fatalf("client send: %v", err)
	}
	dataFrame := server.recv(t, channelReliableOrdered, 5*time.Second)
	ft, id, gotPayload, err := decodeFrame(dataFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameData || id.String() != client.playerID || !bytes.Equal(gotPayload, payload) {
		t.Fatalf("unexpected data frame: type=%d id=%s payload=%q", ft, id, gotPayload)
	}

	reply := []byte("hello from server")
	if err := server.channels[channelReliableOrdered].Send(encodeFrame(frameData, mustParseID(t, client.playerID), reply)); err != nil {
		t.Fatalf("server send: %v", err)
	}
	replyFrame := client.recv(t, channelReliableOrdered, 5*time.Second)
	ft, id, gotReply, err := decodeFrame(replyFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameData || id.String() != server.playerID || !bytes.Equal(gotReply, reply) {
		t.Fatalf("unexpected reply frame: type=%d id=%s payload=%q", ft, id, gotReply)
	}

	if err := client.pc.Close(); err != nil {
		t.Fatalf("client close: %v", err)
	}
	disconnectedFrame := server.recv(t, channelReliableOrdered, 5*time.Second)
	ft, id, _, err = decodeFrame(disconnectedFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != framePlayerDisconnected || id.String() != client.playerID {
		t.Fatalf("expected disconnected frame for %s, got type=%d id=%s", client.playerID, ft, id)
	}
}

func TestSettingsEndpointServesTheServersSettings(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/settings")
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /settings without a server: status %d, want 404", resp.StatusCode)
	}

	hostSettings := `{"map":"simple","countdown-seconds":6}`
	dialRelayWithSettings(t, srv.URL, "server", json.RawMessage(hostSettings))
	// A client's own settings are never stored.
	dialRelayWithSettings(t, srv.URL, "client", json.RawMessage(`{"map":"circuit"}`))

	resp, err = http.Get(srv.URL + "/settings")
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	defer resp.Body.Close() // nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings: status %d, want 200", resp.StatusCode)
	}
	var body struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if string(body.Settings) != hostSettings {
		t.Fatalf("settings = %s, want %s", body.Settings, hostSettings)
	}
}

func TestRelayPreservesSendChannel(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelay(t, srv.URL, "server")
	client := dialRelay(t, srv.URL, "client")
	server.recv(t, channelReliableOrdered, 5*time.Second) // drain connected

	const sendChannel = int(1) // reliable unordered
	payload := []byte("via reliable unordered")
	if err := client.channels[sendChannel].Send(encodeFrame(frameData, mustParseID(t, client.serverPlayerID), payload)); err != nil {
		t.Fatalf("client send: %v", err)
	}

	dataFrame := server.recv(t, sendChannel, 5*time.Second)
	_, _, gotPayload, err := decodeFrame(dataFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("payload = %q, want %q", gotPayload, payload)
	}

	// The frame must not have leaked onto a different channel.
	select {
	case msg := <-server.messages[channelReliableOrdered]:
		t.Fatalf("frame arrived on the wrong channel: %v", msg)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestClientCannotAddressOtherClient(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelay(t, srv.URL, "server")
	clientA := dialRelay(t, srv.URL, "client")
	clientB := dialRelay(t, srv.URL, "client")

	// Drain the two "connected" notifications the server receives.
	server.recv(t, channelReliableOrdered, 5*time.Second)
	server.recv(t, channelReliableOrdered, 5*time.Second)

	payload := []byte("sneaky direct message")
	if err := clientA.channels[channelReliableOrdered].Send(encodeFrame(frameData, mustParseID(t, clientB.playerID), payload)); err != nil {
		t.Fatalf("clientA send: %v", err)
	}

	select {
	case msg := <-clientB.messages[channelReliableOrdered]:
		t.Fatalf("clientB unexpectedly received a message from another client: %v", msg)
	case <-time.After(500 * time.Millisecond):
	}

	// The server, however, can still address clientB directly.
	reply := []byte("server to clientB")
	if err := server.channels[channelReliableOrdered].Send(encodeFrame(frameData, mustParseID(t, clientB.playerID), reply)); err != nil {
		t.Fatalf("server send: %v", err)
	}
	replyFrame := clientB.recv(t, channelReliableOrdered, 5*time.Second)
	ft, id, gotReply, err := decodeFrame(replyFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameData || id.String() != server.playerID || !bytes.Equal(gotReply, reply) {
		t.Fatalf("unexpected reply frame: type=%d id=%s payload=%q", ft, id, gotReply)
	}
}

func putNetsim(t *testing.T, srvURL, playerID string, profile Profile) int {
	t.Helper()
	body, err := json.Marshal(netsimRequest{PlayerID: playerID, Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPut, srvURL+"/netsim", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /netsim: %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestNetsimEndpointRules(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	profile := Profile{LatencyMs: 40}
	if status := putNetsim(t, srv.URL, uuid.NewString(), profile); status != http.StatusNotFound {
		t.Fatalf("PUT without a server: status %d, want 404", status)
	}

	server := dialRelay(t, srv.URL, "server")
	client := dialRelay(t, srv.URL, "client")

	if status := putNetsim(t, srv.URL, client.playerID, profile); status != http.StatusForbidden {
		t.Fatalf("PUT by a client: status %d, want 403", status)
	}
	if status := putNetsim(t, srv.URL, server.playerID, Profile{LossPercent: 101}); status != http.StatusBadRequest {
		t.Fatalf("PUT out of range: status %d, want 400", status)
	}
	if status := putNetsim(t, srv.URL, server.playerID, profile); status != http.StatusNoContent {
		t.Fatalf("PUT by the server: status %d, want 204", status)
	}

	resp, err := http.Get(srv.URL + "/netsim")
	if err != nil {
		t.Fatalf("GET /netsim: %v", err)
	}
	defer resp.Body.Close() // nolint:errcheck
	var got netsimResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Profile != profile {
		t.Fatalf("profile = %+v, want %+v", got.Profile, profile)
	}
}

func TestNetsimResetsWhenTheServerChanges(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelay(t, srv.URL, "server")
	if status := putNetsim(t, srv.URL, server.playerID, Profile{LatencyMs: 40}); status != http.StatusNoContent {
		t.Fatalf("PUT: status %d, want 204", status)
	}
	dialRelay(t, srv.URL, "server")

	r.mu.Lock()
	profile := r.profile
	r.mu.Unlock()
	if profile != (Profile{}) {
		t.Fatalf("profile after a new server = %+v, want off", profile)
	}
}

func TestNetsimDelaysClientLinksOnly(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelay(t, srv.URL, "server")
	client := dialRelay(t, srv.URL, "client")
	server.recv(t, channelReliableOrdered, 5*time.Second) // drain connected

	const latency = 150 * time.Millisecond
	if status := putNetsim(t, srv.URL, server.playerID, Profile{LatencyMs: int(latency / time.Millisecond)}); status != http.StatusNoContent {
		t.Fatalf("PUT: status %d, want 204", status)
	}

	start := time.Now()
	if err := client.channels[channelReliableOrdered].Send(encodeFrame(frameData, mustParseID(t, client.serverPlayerID), []byte("up"))); err != nil {
		t.Fatal(err)
	}
	server.recv(t, channelReliableOrdered, 5*time.Second)
	if elapsed := time.Since(start); elapsed < latency-10*time.Millisecond {
		t.Fatalf("client to server took %v, want at least %v", elapsed, latency)
	}

	start = time.Now()
	if err := server.channels[channelReliableOrdered].Send(encodeFrame(frameData, mustParseID(t, client.playerID), []byte("down"))); err != nil {
		t.Fatal(err)
	}
	client.recv(t, channelReliableOrdered, 5*time.Second)
	if elapsed := time.Since(start); elapsed < latency-10*time.Millisecond {
		t.Fatalf("server to client took %v, want at least %v", elapsed, latency)
	}
}

func TestNetsimDoesNotDelayControlFrames(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelay(t, srv.URL, "server")
	if status := putNetsim(t, srv.URL, server.playerID, Profile{LatencyMs: 2000}); status != http.StatusNoContent {
		t.Fatalf("PUT: status %d, want 204", status)
	}

	dialRelay(t, srv.URL, "client")
	server.recv(t, channelReliableOrdered, 500*time.Millisecond)
}
