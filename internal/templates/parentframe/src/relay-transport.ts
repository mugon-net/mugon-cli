import { Channel } from "@mugon/sdk/protocol";
import type { SettingValue } from "@mugon/sdk/settings";
import type { NetworkProfile } from "./network-profile";
import type {
  NetworkRole,
  ParentframeTransport,
  TransportHandlers,
  TransportIdentity,
} from "@mugon/sdk/parentframe";

// Relay wire protocol. A frame is [type:1][playerId:36][payload:...]; playerIds are
// the relay-assigned uuids in canonical 36-character form. These values are
// shared with the Go relay (internal/relay/relay.go) — keep them in sync.
const FRAME_TYPE_DATA = 0x01;
const FRAME_TYPE_PLAYER_CONNECTED = 0x02;
const FRAME_TYPE_PLAYER_DISCONNECTED = 0x03;
const FRAME_TYPE_DISCONNECT_PLAYER = 0x04;
const UUID_LEN = 36;

// One data channel per delivery guarantee. The array index is the Channel value
// shared with the SDK and the relay; the label lets the relay pair each channel
// with its counterpart on the other player.
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
  private readonly hostSettings: Record<string, SettingValue>;
  private readonly name: string | undefined;
  private pc: RTCPeerConnection | undefined;
  private dataChannels: RTCDataChannel[] = [];
  // The relay-assigned id of this tab's player, known once connected.
  playerId: string | undefined;

  // `hostSettings` are what this tab's settings panel holds. They are only
  // sent when this player connects as the server; the relay serves them to
  // clients, which fetch them with `fetchHostSettings` before they launch.
  // `name` is this tab's display name; without one the relay numbers the tab.
  constructor(
    offerUrl: string,
    hostSettings: Record<string, SettingValue>,
    name?: string,
  ) {
    this.offerUrl = offerUrl;
    this.hostSettings = hostSettings;
    this.name = name;
  }

  async connect(
    role: NetworkRole,
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
      body: JSON.stringify({
        sdp: pc.localDescription!.sdp,
        role,
        name: this.name,
        settings: role === "server" ? this.hostSettings : undefined,
      }),
    });
    if (!res.ok) {
      throw new Error(`relay responded with ${res.status}`);
    }
    const body = (await res.json()) as {
      sdp: string;
      playerId: string;
      name: string;
      serverPlayerId?: string;
      serverName?: string;
    };

    await pc.setRemoteDescription({ type: "answer", sdp: body.sdp });
    await Promise.all(channels.map(waitForDataChannelOpen));

    this.pc = pc;
    this.dataChannels = channels;
    this.playerId = body.playerId;
    channels.forEach((dc, index) => {
      dc.onmessage = (event) =>
        this.handleMessage(index as Channel, event, handlers);
    });

    // The relay (relay.go) only ever notifies the *server* role when a client
    // connects (`notifyServer`) -- there is no corresponding "you are now
    // connected" frame sent to the client itself. Without this, a client-role
    // player's `onPlayerConnected` never fires for its own connection, so the game's
    // Mugon transport (which waits for an explicit "player-connected" naming the server before
    // considering itself linked -- see mugon-sdk-lightyear's client.rs) never leaves
    // its "connecting" state. We already know the server's id from this same HTTP
    // response, so synthesize the notification here rather than changing the wire
    // protocol.
    if (role === "client" && body.serverPlayerId) {
      handlers.onPlayerConnected({
        id: body.serverPlayerId,
        name: body.serverName ?? body.serverPlayerId,
      });
    }

    return {
      self: { id: body.playerId, name: body.name },
      server: body.serverPlayerId
        ? {
            id: body.serverPlayerId,
            name: body.serverName ?? body.serverPlayerId,
          }
        : undefined,
    };
  }

  send(playerId: string, channel: Channel, data: Uint8Array): void {
    this.dataChannels[channel]?.send(
      encodeFrame(FRAME_TYPE_DATA, playerId, data),
    );
  }

  disconnectPlayer(playerId: string): void {
    // Control frames must arrive, so always use the reliable ordered channel.
    this.dataChannels[Channel.ReliableOrdered]?.send(
      encodeFrame(FRAME_TYPE_DISCONNECT_PLAYER, playerId),
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
      case FRAME_TYPE_PLAYER_CONNECTED:
        handlers.onPlayerConnected({
          id,
          name: new TextDecoder().decode(payload) || id,
        });
        break;
      case FRAME_TYPE_PLAYER_DISCONNECTED:
        handlers.onPlayerDisconnected(id);
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

// Sets the simulated network condition of every client link. The relay only
// accepts it from the current server, identified by its player id.
export async function putNetworkProfile(
  relayUrl: string,
  serverPlayerId: string,
  profile: NetworkProfile,
): Promise<void> {
  const res = await fetch(`${relayUrl}/netsim`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ playerId: serverPlayerId, profile }),
  });
  if (!res.ok) throw new Error(`relay responded with ${res.status}`);
}

// The settings the relay's current server started the game with, or undefined
// while no server is connected. A client launches with these, the way mugon.net
// launches every lobby member with the lobby's values.
export async function fetchHostSettings(
  relayUrl: string,
): Promise<Record<string, SettingValue> | undefined> {
  const res = await fetch(`${relayUrl}/settings`);
  if (res.status === 404) return undefined;
  if (!res.ok) throw new Error(`relay responded with ${res.status}`);
  const body = (await res.json()) as {
    settings?: Record<string, SettingValue>;
  };
  return body.settings ?? {};
}
