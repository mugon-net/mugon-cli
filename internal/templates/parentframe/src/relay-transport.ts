import { Channel } from "@mugon/sdk/protocol";
import type {
  NetworkMode,
  ParentframeTransport,
  TransportHandlers,
  TransportIdentity,
} from "@mugon/sdk/parentframe";

// Relay wire protocol. A frame is [type:1][peerId:36][payload:...]; peerIds are
// the relay-assigned uuids in canonical 36-character form. These values are
// shared with the Go relay (internal/relay/relay.go) — keep them in sync.
const FRAME_TYPE_DATA = 0x01;
const FRAME_TYPE_CONNECTED = 0x02;
const FRAME_TYPE_DISCONNECTED = 0x03;
const FRAME_TYPE_DROP = 0x04;
const UUID_LEN = 36;

// One data channel per delivery guarantee. The array index is the Channel value
// shared with the SDK and the relay; the label lets the relay pair each channel
// with its counterpart on the other peer.
const CHANNEL_CONFIGS: { label: string; init: RTCDataChannelInit }[] = [
  { label: "ro", init: { ordered: true } },
  { label: "ru", init: { ordered: false } },
  { label: "uo", init: { ordered: true, maxRetransmits: 0 } },
  { label: "uu", init: { ordered: false, maxRetransmits: 0 } },
];

// RelayTransport connects to the local `mugon dev` WebRTC relay: it opens one
// data channel per delivery guarantee and translates the relay wire frames into
// the transport events the parentframe expects. This is a dev-only transport;
// the production frontend supplies its own.
export class RelayTransport implements ParentframeTransport {
  private readonly offerUrl: string;
  private pc: RTCPeerConnection | undefined;
  private dataChannels: RTCDataChannel[] = [];

  constructor(offerUrl: string) {
    this.offerUrl = offerUrl;
  }

  async connect(
    mode: NetworkMode,
    handlers: TransportHandlers,
  ): Promise<TransportIdentity> {
    const pc = new RTCPeerConnection();
    const channels = CHANNEL_CONFIGS.map((c) => {
      const dc = pc.createDataChannel(c.label, c.init);
      dc.binaryType = "arraybuffer";
      return dc;
    });

    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    await waitForIceGatheringComplete(pc);

    const res = await fetch(this.offerUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ sdp: pc.localDescription!.sdp, role: mode }),
    });
    if (!res.ok) {
      throw new Error(`relay responded with ${res.status}`);
    }
    const body = (await res.json()) as {
      sdp: string;
      clientId: string;
      serverId?: string;
    };

    await pc.setRemoteDescription({ type: "answer", sdp: body.sdp });
    await Promise.all(channels.map(waitForDataChannelOpen));

    this.pc = pc;
    this.dataChannels = channels;
    channels.forEach((dc, index) => {
      dc.onmessage = (event) =>
        this.handleMessage(index as Channel, event, handlers);
    });

    // The relay (relay.go) only ever notifies the *server* role when a client
    // connects (`notifyServer`) -- there is no corresponding "you are now
    // connected" frame sent to the client itself. Without this, a client-role
    // peer's `onPeerConnected` never fires for its own connection, so the game's
    // Mugon transport (which waits for an explicit "con" naming the server before
    // considering itself linked -- see lightyear_mugon's client.rs) never leaves
    // its "connecting" state. We already know the server's id from this same HTTP
    // response, so synthesize the notification here rather than changing the wire
    // protocol.
    if (mode === "client" && body.serverId) {
      handlers.onPeerConnected(body.serverId);
    }

    return { clientId: body.clientId, serverId: body.serverId };
  }

  send(peerId: string, channel: Channel, data: Uint8Array): void {
    this.dataChannels[channel]?.send(
      encodeFrame(FRAME_TYPE_DATA, peerId, data),
    );
  }

  drop(peerId: string): void {
    // Control frames must arrive, so always use the reliable ordered channel.
    this.dataChannels[Channel.ReliableOrdered]?.send(
      encodeFrame(FRAME_TYPE_DROP, peerId),
    );
  }

  close(): void {
    this.pc?.close();
  }

  private handleMessage(
    channel: Channel,
    event: MessageEvent<ArrayBuffer>,
    handlers: TransportHandlers,
  ) {
    const { type, id, payload } = decodeFrame(event.data);

    switch (type) {
      case FRAME_TYPE_DATA:
        handlers.onData(id, channel, payload);
        break;
      case FRAME_TYPE_CONNECTED:
        handlers.onPeerConnected(id);
        break;
      case FRAME_TYPE_DISCONNECTED:
        handlers.onPeerDisconnected(id);
        break;
    }
  }
}

function waitForIceGatheringComplete(pc: RTCPeerConnection): Promise<void> {
  if (pc.iceGatheringState === "complete") return Promise.resolve();
  return new Promise((resolve) => {
    const check = () => {
      if (pc.iceGatheringState === "complete") {
        pc.removeEventListener("icegatheringstatechange", check);
        resolve();
      }
    };
    pc.addEventListener("icegatheringstatechange", check);
  });
}

function waitForDataChannelOpen(dc: RTCDataChannel): Promise<void> {
  if (dc.readyState === "open") return Promise.resolve();
  return new Promise((resolve, reject) => {
    dc.addEventListener("open", () => resolve(), { once: true });
    dc.addEventListener(
      "error",
      (e) => reject(new Error(`data channel error: ${e}`)),
      { once: true },
    );
  });
}

function encodeFrame(
  type: number,
  id: string,
  payload?: Uint8Array,
): Uint8Array<ArrayBuffer> {
  const frame = new Uint8Array(1 + UUID_LEN + (payload?.length ?? 0));
  frame[0] = type;
  frame.set(new TextEncoder().encode(id), 1);
  if (payload) frame.set(payload, 1 + UUID_LEN);
  return frame;
}

function decodeFrame(data: ArrayBuffer): {
  type: number;
  id: string;
  payload: Uint8Array;
} {
  const bytes = new Uint8Array(data);
  return {
    type: bytes[0],
    id: new TextDecoder().decode(bytes.slice(1, 1 + UUID_LEN)),
    payload: bytes.slice(1 + UUID_LEN),
  };
}
