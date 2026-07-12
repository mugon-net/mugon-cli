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

  useEffect(() => {
    // Construct the Parentframe (which attaches the iframe's load listener)
    // before pointing the iframe at the game, so the "init" handshake is never
    // missed on a fast-loading game.
    parentframeRef.current = new Parentframe({
      gameFrame: iframeRef.current!,
      transport: new RelayTransport(
        `http://localhost:${config.webrtcPort}/offer`,
      ),
      onHandshakeComplete: () =>
        console.log("Handshake with gameframe complete."),
      onProtocolMismatch: (gameProtocol, expected) =>
        alert(
          `Mismatching protocol version: CLI protocol version: '${expected}'; Game protocol version: '${gameProtocol}'`,
        ),
    });
    setIframeSrc(`http://localhost:${config.gameFramePort}`);
    console.log("Parentframe initialized.");
  }, [config.webrtcPort, config.gameFramePort]);

  const connect = (mode: NetworkMode) => {
    setStatus({ kind: "connecting" });
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
      });
  };

  const busy = status.kind !== "idle";

  return (
    <div
      style={{
        height: "100vh",
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        backgroundColor: "#1F1F1F",
        fontFamily: "sans-serif",
      }}
    >
      <style>{buttonCss}</style>
      <div
        style={{
          position: "absolute",
          top: "5vh",
          left: "50%",
          transform: "translateX(-50%)",
          color: "white",
          fontSize: "2rem",
        }}
      >
        Mugon CLI - Dev Server - {config.projectId}
      </div>
      <iframe
        ref={iframeRef}
        src={iframeSrc}
        sandbox="allow-scripts"
        scrolling="no"
        style={{ width: "80vw", height: "80vh", border: "1px solid white" }}
      />
      <div
        style={{
          position: "absolute",
          bottom: "5vh",
          width: "100vw",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          alignItems: "center",
          gap: "0.75rem",
        }}
      >
        <div style={{ display: "flex", gap: "1rem" }}>
          <button
            className="button"
            disabled={busy}
            onClick={() => connect("server")}
          >
            Start as host
          </button>
          <button
            className="button"
            disabled={busy}
            onClick={() => connect("client")}
          >
            Start as client
          </button>
        </div>
        <div
          style={{ color: "#AAAAAA", fontSize: "0.9rem", minHeight: "1.2rem" }}
        >
          {status.kind}
        </div>
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
