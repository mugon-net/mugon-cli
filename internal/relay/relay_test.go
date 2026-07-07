package relay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type testClient struct {
	pc *webrtc.PeerConnection
	dc *webrtc.DataChannel

	messages chan []byte

	clientID string
	serverID string
}

func mustParseID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("uuid.Parse(%q): %v", s, err)
	}
	return id
}

func dialRelay(t *testing.T, srvURL string, role string) *testClient {
	t.Helper()

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	dc, err := pc.CreateDataChannel("relay", nil)
	if err != nil {
		t.Fatalf("CreateDataChannel: %v", err)
	}

	tc := &testClient{pc: pc, dc: dc, messages: make(chan []byte, 16)}
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		tc.messages <- msg.Data
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}
	<-gatherComplete

	reqBody, _ := json.Marshal(offerRequest{SDP: pc.LocalDescription().SDP, Role: role})
	resp, err := http.Post(srvURL+"/offer", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST /offer: %v", err)
	}
	defer resp.Body.Close()
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

	waitOpen := make(chan struct{})
	dc.OnOpen(func() { close(waitOpen) })
	select {
	case <-waitOpen:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for data channel to open")
	}

	tc.clientID = body.ClientID
	tc.serverID = body.ServerID
	return tc
}

func (tc *testClient) recv(t *testing.T, timeout time.Duration) []byte {
	t.Helper()
	select {
	case msg := <-tc.messages:
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func TestRelayEndToEnd(t *testing.T) {
	r := New()
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	server := dialRelay(t, srv.URL, "server")
	if server.clientID == "" {
		t.Fatal("server did not receive a clientId")
	}

	client := dialRelay(t, srv.URL, "client")
	if client.clientID == "" {
		t.Fatal("client did not receive a clientId")
	}
	if client.serverID != server.clientID {
		t.Fatalf("client.serverID = %q, want %q", client.serverID, server.clientID)
	}

	connectedFrame := server.recv(t, 5*time.Second)
	ft, id, _, err := decodeFrame(connectedFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameConnected || id.String() != client.clientID {
		t.Fatalf("expected connected frame for %s, got type=%d id=%s", client.clientID, ft, id)
	}

	payload := []byte("hello from client")
	if err := client.dc.Send(encodeFrame(frameData, mustParseID(t, client.serverID), payload)); err != nil {
		t.Fatalf("client send: %v", err)
	}
	dataFrame := server.recv(t, 5*time.Second)
	ft, id, gotPayload, err := decodeFrame(dataFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameData || id.String() != client.clientID || !bytes.Equal(gotPayload, payload) {
		t.Fatalf("unexpected data frame: type=%d id=%s payload=%q", ft, id, gotPayload)
	}

	reply := []byte("hello from server")
	if err := server.dc.Send(encodeFrame(frameData, mustParseID(t, client.clientID), reply)); err != nil {
		t.Fatalf("server send: %v", err)
	}
	replyFrame := client.recv(t, 5*time.Second)
	ft, id, gotReply, err := decodeFrame(replyFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameData || id.String() != server.clientID || !bytes.Equal(gotReply, reply) {
		t.Fatalf("unexpected reply frame: type=%d id=%s payload=%q", ft, id, gotReply)
	}

	if err := client.pc.Close(); err != nil {
		t.Fatalf("client close: %v", err)
	}
	disconnectedFrame := server.recv(t, 5*time.Second)
	ft, id, _, err = decodeFrame(disconnectedFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameDisconnected || id.String() != client.clientID {
		t.Fatalf("expected disconnected frame for %s, got type=%d id=%s", client.clientID, ft, id)
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
	server.recv(t, 5*time.Second)
	server.recv(t, 5*time.Second)

	payload := []byte("sneaky direct message")
	if err := clientA.dc.Send(encodeFrame(frameData, mustParseID(t, clientB.clientID), payload)); err != nil {
		t.Fatalf("clientA send: %v", err)
	}

	select {
	case msg := <-clientB.messages:
		t.Fatalf("clientB unexpectedly received a message from another client: %v", msg)
	case <-time.After(500 * time.Millisecond):
	}

	// The server, however, can still address clientB directly.
	reply := []byte("server to clientB")
	if err := server.dc.Send(encodeFrame(frameData, mustParseID(t, clientB.clientID), reply)); err != nil {
		t.Fatalf("server send: %v", err)
	}
	replyFrame := clientB.recv(t, 5*time.Second)
	ft, id, gotReply, err := decodeFrame(replyFrame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if ft != frameData || id.String() != server.clientID || !bytes.Equal(gotReply, reply) {
		t.Fatalf("unexpected reply frame: type=%d id=%s payload=%q", ft, id, gotReply)
	}
}
