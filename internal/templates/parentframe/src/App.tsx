import {
  createVersionFileFetcher,
  LaunchPhase,
  LoadProgress,
  NetworkMode,
  NoServerAvailableError,
  Parentframe,
  sdkMajor,
} from "@mugon/sdk/parentframe";
import { useEffect, useRef, useState } from "react";
import { RelayTransport } from "./relay-transport";

export interface MugonConfig {
  projectId: string;
  jsSdkVersion: string;
  webrtcPort: number;
  gameFramePort: number;
}

type Status =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "running" }
  | { kind: "no-server" }
  | { kind: "error"; message: string };

const CACHE_PREFIX = "mugon-dev-";

// The dev server's identifier of the current build; caches of older builds are
// dropped so a rebuilt game never runs on stale files.
async function readBuildInfo(): Promise<{ id: string; totalBytes: number }> {
  const build = await (await fetch("/__mugon/build")).json();
  if (typeof caches !== "undefined") {
    for (const name of await caches.keys()) {
      if (name.startsWith(CACHE_PREFIX) && name !== `${CACHE_PREFIX}${build.id}`) {
        await caches.delete(name);
      }
    }
  }
  return build;
}

const PHASE_LABELS: Record<Exclude<LaunchPhase, "running">, string> = {
  "frame-created": "Game frame created",
  "network-ready": "Network connected",
  "hello-received": "Game SDK responded",
};

export function App({ config }: { config: MugonConfig }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const parentframeRef = useRef<Parentframe | null>(null);
  const declaredMajor = Number(config.jsSdkVersion.split(".")[0]);
  const [status, setStatus] = useState<Status>(
    declaredMajor === sdkMajor
      ? { kind: "idle" }
      : {
          kind: "error",
          message: `mugon.toml declares js-sdk-version major ${declaredMajor}, but this CLI only supports major ${sdkMajor}. Update the CLI or the js-sdk-version.`,
        },
  );
  const [phases, setPhases] = useState<LaunchPhase[]>([]);
  const [progress, setProgress] = useState<LoadProgress>({
    loadedBytes: 0,
    done: false,
  });

  useEffect(() => () => parentframeRef.current?.destroy(), []);

  const fail = (error: unknown) => {
    if (error instanceof NoServerAvailableError) {
      setStatus({ kind: "no-server" });
    } else {
      setStatus({
        kind: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  };

  const launch = async (mode: NetworkMode) => {
    setStatus({ kind: "loading" });
    try {
      const gameUrl = `${location.protocol}//${location.hostname}:${config.gameFramePort}/`;
      const build = await readBuildInfo();
      const parentframe = new Parentframe({
        transport: new RelayTransport(
          `${location.protocol}//${location.hostname}:${config.webrtcPort}/offer`,
        ),
        fetchVersionFile: createVersionFileFetcher({
          versionBaseUrl: gameUrl,
          cacheKeyBase: gameUrl,
          cacheName: `${CACHE_PREFIX}${build.id}`,
        }),
        onPhase: (phase) => setPhases((previous) => [...previous, phase]),
        onProgress: setProgress,
      });
      parentframeRef.current = parentframe;
      await parentframe.launch({
        container: containerRef.current!,
        src: gameUrl,
        mode,
        totalBytes: build.totalBytes,
      });
      setStatus({ kind: "running" });
    } catch (error) {
      fail(error);
    }
  };

  const retryNetwork = () => {
    setStatus({ kind: "loading" });
    parentframeRef
      .current!.retryNetwork()
      .then(() => setStatus({ kind: "running" }))
      .catch(fail);
  };

  const started = status.kind !== "idle" && status.kind !== "error";
  const showOverlay =
    status.kind === "loading" ||
    (status.kind === "running" && !progress.done);

  return (
    // A real flex column rather than `vh`-offset overlays: on a short viewport
    // (a landscape phone) the game area shrinks instead of the title drawing
    // over it.
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
        <div
          ref={containerRef}
          style={{
            width: "100%",
            height: "100%",
            border: "1px solid white",
            boxSizing: "border-box",
            display: started ? "block" : "none",
          }}
        />
        {status.kind === "idle" && (
          <CenteredOverlay>
            <button className="button" onClick={() => launch("server")}>
              Start as host
            </button>
            <button className="button" onClick={() => launch("client")}>
              Start as client
            </button>
          </CenteredOverlay>
        )}
        {showOverlay && (
          <CenteredOverlay column background>
            {(Object.keys(PHASE_LABELS) as (keyof typeof PHASE_LABELS)[]).map(
              (phase) => (
                <div
                  key={phase}
                  style={{ color: phases.includes(phase) ? "#7BD88F" : "#777" }}
                >
                  {phases.includes(phase) ? "✓" : "…"} {PHASE_LABELS[phase]}
                </div>
              ),
            )}
            <ProgressBar progress={progress} />
          </CenteredOverlay>
        )}
        {status.kind === "no-server" && (
          <CenteredOverlay column background>
            <div style={{ color: "white" }}>
              No server has been started yet. Start a host first.
            </div>
            <button className="button" onClick={retryNetwork}>
              Retry
            </button>
          </CenteredOverlay>
        )}
        {status.kind === "error" && (
          <CenteredOverlay column background>
            <div style={{ color: "#FF8A80", maxWidth: "40rem" }}>
              {status.message}
            </div>
            <button className="button" onClick={() => location.reload()}>
              Reload
            </button>
          </CenteredOverlay>
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
        {started ? status.kind : ""}
      </div>
    </div>
  );
}

function CenteredOverlay(props: {
  children: React.ReactNode;
  column?: boolean;
  background?: boolean;
}) {
  return (
    <div
      style={{
        position: "absolute",
        inset: 0,
        display: "flex",
        flexDirection: props.column ? "column" : "row",
        alignItems: "center",
        justifyContent: "center",
        gap: "1rem",
        textAlign: "center",
        color: "#DDD",
        backgroundColor: props.background ? "rgba(31, 31, 31, 0.92)" : undefined,
      }}
    >
      {props.children}
    </div>
  );
}

function ProgressBar({ progress }: { progress: LoadProgress }) {
  const percent =
    progress.fraction === undefined ? undefined : progress.fraction * 100;
  return (
    <div style={{ width: "20rem" }}>
      <div
        style={{
          height: "0.5rem",
          backgroundColor: "#333",
          borderRadius: "0.25rem",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            height: "100%",
            width: percent === undefined ? "30%" : `${percent}%`,
            backgroundColor: "#7BD88F",
            opacity: percent === undefined ? 0.4 : 1,
            transition: "width 0.2s ease",
          }}
        />
      </div>
      <div style={{ marginTop: "0.5rem", fontSize: "0.85rem", color: "#AAA" }}>
        {progress.label ?? (percent === undefined ? "Loading…" : "")}
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
