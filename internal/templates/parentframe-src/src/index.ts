import {
  FromGameframeMessage,
  PROTOCOL,
  ToGameframeDataMessage,
  ToGrameframeConnectedMessage,
  ToGrameframeDisconnectedMessage,
} from "@mugon/sdk";

declare global {
  interface Window {
    __MUGON_WEBRTC_PORT__?: number;
  }
}

type NetworkMode = "server" | "client";

const FRAME_TYPE_DATA = 0x01;
const FRAME_TYPE_CONNECTED = 0x02;
const FRAME_TYPE_DISCONNECTED = 0x03;
const FRAME_TYPE_DROP = 0x04;
const UUID_LEN = 36;

let port: MessagePort | undefined;
let dataChannel: RTCDataChannel | undefined;
let startServerButton: HTMLButtonElement;
let joinServerButton: HTMLButtonElement;

const settings: Record<string, string | undefined> = {
  "mugon.networkmode": undefined,
  "mugon.serverid": undefined,
  "mugon.clientid": undefined,
};

function init() {
  const channel = new MessageChannel();
  port = channel.port1;

  const iframe = document.querySelector("iframe")!;
  iframe.addEventListener("load", () => {
    iframe.contentWindow!.postMessage({ type: "init" }, "*", [channel.port2]);
  });
  port.onmessage = handleChildFrameMessage;

  startServerButton = document.getElementById(
    "start-server-button",
  ) as HTMLButtonElement;
  joinServerButton = document.getElementById(
    "join-server-button",
  ) as HTMLButtonElement;

  startServerButton.addEventListener("click", () => {
    startServerButton.disabled = true;
    joinServerButton.disabled = true;
    connect("server").catch((err) => {
      alert(`Failed to connect to the local webrtc relay: ${err}`);
      startServerButton.disabled = false;
      joinServerButton.disabled = false;
    });
  });
  joinServerButton.addEventListener("click", () => {
    startServerButton.disabled = true;
    joinServerButton.disabled = true;
    connect("client").catch((err) => {
      alert(`Failed to connect to the local webrtc relay: ${err}`);
      startServerButton.disabled = false;
      joinServerButton.disabled = false;
    });
  });

  console.log("Parentframe initialized.");
}

async function connect(mode: NetworkMode) {
  const relayPort = window.__MUGON_WEBRTC_PORT__;
  if (!relayPort) {
    throw new Error("webrtc relay port was not injected into the page");
  }

  const { pc, dc, clientId, serverId } = await connectToRelay(relayPort, mode);

  if (mode === "client" && !serverId) {
    pc.close();
    alert("No server has been started yet. Start a server before joining.");
    startServerButton.disabled = false;
    joinServerButton.disabled = false;
    return;
  }

  dataChannel = dc;
  dataChannel.onmessage = handleRelayMessage;

  settings["mugon.networkmode"] = mode;
  if (mode === "server") {
    settings["mugon.serverid"] = clientId;
  } else {
    settings["mugon.clientid"] = clientId;
    settings["mugon.serverid"] = serverId;
  }

  sendSettings();
  setTimeout(sendReady, 250);
}

async function connectToRelay(
  relayPort: number,
  role: NetworkMode,
): Promise<{
  pc: RTCPeerConnection;
  dc: RTCDataChannel;
  clientId: string;
  serverId?: string;
}> {
  const pc = new RTCPeerConnection();
  const dc = pc.createDataChannel("relay");

  const offer = await pc.createOffer();
  await pc.setLocalDescription(offer);
  await waitForIceGatheringComplete(pc);

  const res = await fetch(`http://localhost:${relayPort}/offer`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ sdp: pc.localDescription!.sdp, role }),
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
  await waitForDataChannelOpen(dc);

  return { pc, dc, clientId: body.clientId, serverId: body.serverId };
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

function handleRelayMessage(event: MessageEvent<ArrayBuffer>) {
  const { type, id, payload } = decodeFrame(event.data);

  switch (type) {
    case FRAME_TYPE_DATA: {
      const msg: ToGameframeDataMessage = {
        type: "msg",
        fromclientid: id,
        data: payload,
      };
      port!.postMessage(msg, [payload.buffer]);
      break;
    }
    case FRAME_TYPE_CONNECTED: {
      const msg: ToGrameframeConnectedMessage = { type: "con", clientid: id };
      port!.postMessage(msg);
      break;
    }
    case FRAME_TYPE_DISCONNECTED: {
      const msg: ToGrameframeDisconnectedMessage = { type: "dc", clientid: id };
      port!.postMessage(msg);
      break;
    }
  }
}

function handleChildFrameMessage(event: MessageEvent<FromGameframeMessage>) {
  const msg = event.data;

  switch (msg.type) {
    case "rdy":
      if (msg.protocol !== PROTOCOL) {
        alert(
          `Mismatching protocol version: CLI protocol version: '${PROTOCOL}'; Game protocol version: '${msg.protocol}'`,
        );
      } else {
        console.log(
          `Handshake with gameframe complete. Protocol: '${PROTOCOL}'`,
        );
      }
      break;
    case "msg":
      dataChannel?.send(encodeFrame(FRAME_TYPE_DATA, msg.toclientid, msg.data));
      break;
    case "drp":
      dataChannel?.send(encodeFrame(FRAME_TYPE_DROP, msg.clientid));
      break;
  }
}

function sendSettings() {
  port!.postMessage({ type: "set", settings });
}

function sendReady() {
  port!.postMessage({ type: "rdy", protocol: PROTOCOL });
}

init();
