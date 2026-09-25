import {
  NetworkMode,
  NoServerAvailableError,
  Parentframe,
} from "@mugon/sdk/parentframe";
import { useEffect, useRef, useState } from "react";
import { RelayTransport } from "./relay-transport";

export interface MugonConfig {
  projectId: string;
  webrtcPort: number;
  gameFramePort: number;
}

type Status =
  | { kind: "idle" }
  | { kind: "connecting" }
  | { kind: "connected"; mode: NetworkMode };

export function App({ config }: { config: MugonConfig }) {
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const parentframeRef = useRef<Parentframe | null>(null);
  const [status, setStatus] = useState<Status>({ kind: "idle" });
  const [iframeSrc, setIframeSrc] = useState<string>();
  const [roleChosen, setRoleChosen] = useState(false);

  useEffect(() => {
    // Construct the Parentframe (which attaches the iframe's `load` listener)
    // right away, but leave the iframe's `src` unset until a role is chosen (see
    // `connect` below) -- the game itself (its wasm, currently ~120MB+) shouldn't
    // fetch or start running before the user has actually picked "Start as
    // host"/"Start as client". A srcless iframe never navigates, so `load` simply
    // never fires (and `establishChildHandshake`'s retry loop never starts) until
    // `connect` sets a real `src`, at which point the handshake proceeds exactly as
    // it did when this ran eagerly on mount.
    parentframeRef.current = new Parentframe({
      gameFrame: iframeRef.current!,
      transport: new RelayTransport(
        `${location.protocol}//${location.hostname}:${config.webrtcPort}/offer`,
      ),
      onHandshakeComplete: () =>
        console.log("Handshake with gameframe complete."),
      onProtocolMismatch: (gameProtocol, expected) =>
        alert(
          `Mismatching protocol version: CLI protocol version: '${expected}'; Game protocol version: '${gameProtocol}'`,
        ),
    });
    console.log("Parentframe initialized.");
  }, [config.webrtcPort, config.gameFramePort]);

  const connect = (mode: NetworkMode) => {
    setRoleChosen(true);
    setStatus({ kind: "connecting" });
    // First real navigation of the iframe -- this is the moment the game's wasm
    // actually starts downloading, not page load.
    setIframeSrc(
      `${location.protocol}//${location.hostname}:${config.gameFramePort}`,
    );
    parentframeRef
      .current!.connect(mode)
      .then(() => setStatus({ kind: "connected", mode }))
      .catch((err) => {
        if (err instanceof NoServerAvailableError) {
          alert(
            "No server has been started yet. Start a server before joining.",
          );
        } else {
          alert(`Failed to connect to the local webrtc relay: ${err}`);
        }
        setStatus({ kind: "idle" });
        setRoleChosen(false);
      });
  };

  return (
    // A real flex column, not three independently `position: absolute`-guessed
    // overlays -- the title and status row used to be laid out purely by `vh`
    // offsets from the viewport edges, which assumed enough clearance before the
    // (separately, flex-centered) iframe. That assumption broke on a short
    // viewport (a landscape phone): the title's fixed-size text no longer fit in
    // its `vh`-sized gap and started drawing over the iframe. A normal column
    // gives the title and status row their own space in document flow, with the
    // iframe (`flex: 1 1 auto`) filling whatever's left -- the game area is what
    // shrinks on a short viewport, not something the title can ever overlap.
    <div
      style={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        backgroundColor: "#1F1F1F",
        fontFamily: "sans-serif",
      }}
    >
      <style>{buttonCss}</style>
      <div
        style={{
          flexShrink: 0,
          padding: "1rem 0",
          color: "white",
          fontSize: "1.5rem",
          textAlign: "center",
        }}
      >
        Mugon CLI - Dev Server - {config.projectId}
      </div>
      <div
        style={{
          position: "relative",
          flex: "1 1 auto",
          minHeight: 0,
          width: "80vw",
        }}
      >
        <iframe
          ref={iframeRef}
          src={iframeSrc}
          sandbox="allow-scripts allow-pointer-lock allow-fullscreen"
          allow="fullscreen"
          scrolling="no"
          style={{
            width: "100%",
            height: "100%",
            border: "1px solid white",
            display: roleChosen ? "block" : "none",
          }}
        />
        {!roleChosen && (
          <div
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "1rem",
            }}
          >
            <button className="button" onClick={() => connect("server")}>
              Start as host
            </button>
            <button className="button" onClick={() => connect("client")}>
              Start as client
            </button>
          </div>
        )}
      </div>
      <div
        style={{
          flexShrink: 0,
          padding: "1rem 0",
          color: "#AAAAAA",
          fontSize: "0.9rem",
          minHeight: "1.2rem",
          textAlign: "center",
        }}
      >
        {roleChosen ? status.kind : ""}
      </div>
    </div>
  );
}

const buttonCss = `
.button {
  padding: 0.75rem 1.75rem;
  font-family: sans-serif;
  font-size: 1rem;
  font-weight: 600;
  color: white;
  background-color: #2E2E2E;
  border: 1px solid #3F3F3F;
  border-radius: 8px;
  cursor: pointer;
  transition: background-color 0.15s ease;
}
.button:hover {
  background-color: #3A3A3A;
}
.button:active {
  background-color: #232323;
}
.button:disabled {
  color: #777777;
  background-color: #262626;
  border-color: #333333;
  cursor: not-allowed;
}
`;
