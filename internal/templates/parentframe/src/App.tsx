import {
  createVersionFileFetcher,
  LaunchPhase,
  LoadProgress,
  NetworkRole,
  NoServerAvailableError,
  Parentframe,
  sdkMajor,
} from "@mugon/sdk/parentframe";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  areSettingsValid,
  compileSettingExpressions,
} from "@mugon/sdk/expression";
import {
  SettingDeclaration,
  SettingValue,
  settingValuesFromQuery,
} from "@mugon/sdk/settings";
import { resolveNextRoundSettings } from "./next-round";
import { NetworkBar } from "./network-bar";
import { NETWORK_OFF, NetworkProfile } from "./network-profile";
import {
  fetchHostSettings,
  putNetworkProfile,
  RelayTransport,
} from "./relay-transport";
import { SettingsPanel } from "./settings-panel";

const MAX_SIM_CLIENTS = 7;
const SIM_READY_MESSAGE = "mugon-sim-ready";
const SIM_LAUNCH_MESSAGE = "mugon-sim-launch";

export interface MugonConfig {
  projectId: string;
  jsSdkVersion: string;
  webrtcPort: number;
  gameFramePort: number;
  settings: SettingDeclaration[];
  settingsValidWhen: string | null;
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
      if (
        name.startsWith(CACHE_PREFIX) &&
        name !== `${CACHE_PREFIX}${build.id}`
      ) {
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
  // Seeded from the declared defaults with any `s.<key>` query parameter
  // applied on top, so a dev can bookmark a specific settings combination.
  const [settingValues, setSettingValues] = useState<
    Record<string, SettingValue>
  >(() =>
    settingValuesFromQuery(
      config.settings,
      new URLSearchParams(window.location.search),
    ),
  );
  const declaredMajor = Number(config.jsSdkVersion.split(".")[0]);
  const [status, setStatus] = useState<Status>(
    declaredMajor === sdkMajor
      ? { kind: "idle" }
      : {
          kind: "error",
          message: `mugon.toml declares js-sdk-version major ${declaredMajor}, but this CLI only supports major ${sdkMajor}. Update the CLI or the js-sdk-version.`,
        },
  );
  // What the game proposed for the next round, awaiting confirmation.
  const [nextRound, setNextRound] = useState<{
    settings: Record<string, SettingValue>;
    carryOver?: string;
  }>();
  // Why the game is back at the start screen, when the game caused it.
  const [notice, setNotice] = useState<string>();
  const [phases, setPhases] = useState<LaunchPhase[]>([]);
  const [progress, setProgress] = useState<LoadProgress>({
    loadedBytes: 0,
    done: false,
  });

  // A simulated client is an iframe of this page the host tab embeds: it has no
  // start screen and launches as a client when the host tab tells it to.
  const simClientIndex = new URLSearchParams(location.search).get("sim-client");
  const isSimClient = simClientIndex !== null;
  const [activeRole, setActiveRole] = useState<NetworkRole>();
  const [simClientCount, setSimClientCount] = useState(0);
  const [simClientsActive, setSimClientsActive] = useState(false);
  // Keeps the names of simulated clients from different tabs apart.
  const [tabTag] = useState(() => Math.random().toString(16).slice(2, 6));
  const simClientName = (number: number) =>
    activeRole === "server" ? `Sim ${number}` : `Sim ${tabTag}-${number}`;
  const simTileRefs = useRef<(HTMLIFrameElement | null)[]>([]);
  // A browser runs all frames of one site on a single thread, and the ports of
  // `localhost` do not make sites apart, so tiles on the same host would split
  // one thread between all their games. Each `*.localhost` name is its own site.
  const simClientSrc = (number: number) => {
    const isLocalhost =
      location.hostname === "localhost" ||
      location.hostname.endsWith(".localhost");
    const origin = isLocalhost
      ? `${location.protocol}//sim-${tabTag}-${number}.localhost${location.port ? `:${location.port}` : ""}`
      : location.origin;
    const query = new URLSearchParams({
      "sim-client": String(number),
      name: simClientName(number),
      "parent-origin": location.origin,
    });
    return `${origin}${location.pathname}?${query}`;
  };
  // Lives in this tab (not the relay) so it survives next rounds; a reload resets it.
  const [networkProfile, setNetworkProfile] =
    useState<NetworkProfile>(NETWORK_OFF);
  const networkProfileRef = useRef<NetworkProfile>(NETWORK_OFF);
  const hostTransportRef = useRef<RelayTransport | null>(null);

  const relayUrl = `${location.protocol}//${location.hostname}:${config.webrtcPort}`;

  const applyNetworkProfile = () => {
    const playerId = hostTransportRef.current?.playerId;
    if (!playerId) return;
    putNetworkProfile(relayUrl, playerId, networkProfileRef.current).catch(
      (error) => console.error("Applying the network profile failed:", error),
    );
  };

  const changeNetworkProfile = (profile: NetworkProfile) => {
    networkProfileRef.current = profile;
    setNetworkProfile(profile);
    applyNetworkProfile();
  };

  useEffect(() => () => parentframeRef.current?.destroy(), []);

  useEffect(() => {
    const parentOrigin = new URLSearchParams(location.search).get(
      "parent-origin",
    );
    const onMessage = (event: MessageEvent) => {
      if (isSimClient) {
        if (
          event.source === window.parent &&
          event.origin === parentOrigin &&
          event.data?.type === SIM_LAUNCH_MESSAGE
        ) {
          void launch("client");
        }
      } else if (
        event.data?.type === SIM_READY_MESSAGE &&
        simTileRefs.current.some((tile) => tile?.contentWindow === event.source)
      ) {
        (event.source as Window).postMessage(
          { type: SIM_LAUNCH_MESSAGE },
          event.origin,
        );
      }
    };
    window.addEventListener("message", onMessage);
    if (isSimClient && parentOrigin) {
      window.parent.postMessage({ type: SIM_READY_MESSAGE }, parentOrigin);
    }
    return () => window.removeEventListener("message", onMessage);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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

  // Parsed once; the dev server injects the expressions from mugon.toml, so a
  // bad one shows up here rather than at publish time.
  const settingExpressions = useMemo(
    () =>
      compileSettingExpressions(
        config.settings,
        config.settingsValidWhen ?? undefined,
      ),
    [config.settings, config.settingsValidWhen],
  );
  const settingsValid = areSettingsValid(settingExpressions, settingValues);

  // The dev harness has no lobby: the start screen, with the settings panel
  // still filled in, stands in for it.
  const returnToStart = (reason?: string) => {
    parentframeRef.current?.destroy();
    parentframeRef.current = null;
    hostTransportRef.current = null;
    setActiveRole(undefined);
    setSimClientsActive(false);
    setNextRound(undefined);
    setPhases([]);
    setProgress({ loadedBytes: 0, done: false });
    setNotice(reason);
    setStatus({ kind: "idle" });
  };

  const continueToNextRound = () => {
    const resolution = resolveNextRoundSettings(
      config.settings,
      settingExpressions,
      nextRound?.settings ?? {},
    );
    if ("error" in resolution) {
      returnToStart(
        `Continuing to the next round failed: ${resolution.error}. The platform would send everyone back to the lobby.`,
      );
      return;
    }
    returnToStart();
    setSettingValues(resolution.values);
    void launch("server", resolution.values, nextRound?.carryOver);
  };

  const launch = async (
    role: NetworkRole,
    hostSettings: Record<string, SettingValue> = settingValues,
    carryOver?: string,
  ) => {
    setNotice(undefined);
    setStatus({ kind: "loading" });
    try {
      const gameUrl = `${location.protocol}//${location.hostname}:${config.gameFramePort}/`;
      // A client plays with whatever the host started with and never uses its
      // own panel, so there is nothing to launch until a host exists.
      const launchSettings =
        role === "client" ? await fetchHostSettings(relayUrl) : hostSettings;
      if (!launchSettings) {
        setStatus({ kind: "no-server" });
        return;
      }
      const build = await readBuildInfo();
      const transport = new RelayTransport(
        `${relayUrl}/offer`,
        hostSettings,
        new URLSearchParams(location.search).get("name") ?? undefined,
      );
      if (role === "server") hostTransportRef.current = transport;
      const parentframe = new Parentframe({
        transport,
        fetchVersionFile: createVersionFileFetcher({
          versionBaseUrl: gameUrl,
          cacheKeyBase: gameUrl,
          cacheName: `${CACHE_PREFIX}${build.id}`,
        }),
        onPhase: (phase) => setPhases((previous) => [...previous, phase]),
        onProgress: setProgress,
        onEndGameRequest: () => {
          if (isSimClient) return;
          returnToStart(
            "The game ended. The platform would return everyone to the lobby.",
          );
        },
        onNextRoundRequest: (settings, carryOver) => {
          if (isSimClient) return;
          setNextRound({ settings, carryOver });
        },
      });
      parentframeRef.current = parentframe;
      await parentframe.launch({
        container: containerRef.current!,
        src: gameUrl,
        role,
        settings: launchSettings,
        carryOver,
        totalBytes: build.totalBytes,
      });
      setActiveRole(role);
      if (role === "server") applyNetworkProfile();
      if (!isSimClient) setSimClientsActive(true);
      setStatus({ kind: "running" });
    } catch (error) {
      fail(error);
    }
  };

  // Before the game frame exists (no host was running at launch) there is
  // nothing to retry but the launch itself.
  const retry = () => {
    if (!parentframeRef.current) {
      void launch("client");
      return;
    }
    retryNetwork();
  };

  const retryNetwork = () => {
    setStatus({ kind: "loading" });
    parentframeRef
      .current!.retryNetwork()
      .then(() => {
        applyNetworkProfile();
        setStatus({ kind: "running" });
      })
      .catch(fail);
  };

  const started = status.kind !== "idle" && status.kind !== "error";
  const showOverlay =
    status.kind === "loading" || (status.kind === "running" && !progress.done);

  if (isSimClient) {
    return (
      <div
        style={{
          position: "relative",
          height: "100vh",
          backgroundColor: "#1F1F1F",
          fontFamily: "sans-serif",
        }}
      >
        <div
          ref={containerRef}
          style={{
            width: "100%",
            height: "100%",
            display:
              status.kind === "idle" || status.kind === "error"
                ? "none"
                : "block",
          }}
        />
        {status.kind !== "running" && (
          <CenteredOverlay column>
            <div style={{ fontSize: "0.8rem" }}>
              {status.kind === "error"
                ? status.message
                : status.kind === "no-server"
                  ? "No host"
                  : `Sim ${simClientIndex}: connecting…`}
            </div>
          </CenteredOverlay>
        )}
      </div>
    );
  }

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
          <CenteredOverlay column>
            {notice && (
              <div style={{ color: "#FFD180", maxWidth: "40rem" }}>
                {notice}
              </div>
            )}
            <div style={{ display: "flex", gap: "1rem" }}>
              <button
                className="button"
                disabled={!settingsValid}
                title={
                  settingsValid
                    ? undefined
                    : `Settings are not valid yet: ${config.settingsValidWhen}`
                }
                onClick={() => launch("server")}
              >
                Start as host
              </button>
              <button className="button" onClick={() => launch("client")}>
                Start as client
              </button>
            </div>
            <label
              style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}
              title="Extra game instances that join the host as clients, shown as tiles under the game"
            >
              <span>Simulated clients</span>
              <input
                type="number"
                min={0}
                max={MAX_SIM_CLIENTS}
                value={simClientCount}
                style={{ width: "4rem" }}
                onChange={(event) =>
                  setSimClientCount(
                    Math.min(
                      MAX_SIM_CLIENTS,
                      Math.max(0, Math.trunc(event.target.valueAsNumber) || 0),
                    ),
                  )
                }
              />
            </label>
            <SettingsPanel
              declarations={config.settings}
              values={settingValues}
              expressions={settingExpressions}
              onChange={(key, value) =>
                setSettingValues((current) => ({ ...current, [key]: value }))
              }
            />
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
        {nextRound && (
          <CenteredOverlay column background>
            <div style={{ color: "white", fontSize: "1.25rem" }}>
              Continue to next round?
            </div>
            <div style={{ display: "flex", gap: "1rem" }}>
              <button className="button" onClick={continueToNextRound}>
                Continue
              </button>
              <button className="button" onClick={() => returnToStart()}>
                Back to start
              </button>
            </div>
          </CenteredOverlay>
        )}
        {status.kind === "no-server" && (
          <CenteredOverlay column background>
            <div style={{ color: "white" }}>
              No server has been started yet. Start a host first.
            </div>
            <button className="button" onClick={retry}>
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
      {simClientsActive && simClientCount > 0 && (
        <div
          style={{
            flexShrink: 0,
            display: "flex",
            flexWrap: "wrap",
            justifyContent: "center",
            gap: "0.5rem",
            paddingTop: "0.5rem",
          }}
        >
          {Array.from({ length: simClientCount }, (_, index) => (
            <iframe
              key={index}
              ref={(tile) => {
                simTileRefs.current[index] = tile;
              }}
              title={`Simulated client ${index + 1}`}
              src={simClientSrc(index + 1)}
              style={{
                width: "15rem",
                height: "9.5rem",
                border: "1px solid #555",
              }}
            />
          ))}
        </div>
      )}
      <div
        style={{
          flexShrink: 0,
          padding: "1rem 0",
          color: "#AAAAAA",
          fontSize: "0.9rem",
          minHeight: "1.2rem",
          textAlign: "center",
          display: "flex",
          flexDirection: "column",
          gap: "0.5rem",
        }}
      >
        {activeRole === "server" && status.kind === "running" && (
          <NetworkBar
            profile={networkProfile}
            onChange={changeNetworkProfile}
          />
        )}
        <div>{started ? status.kind : ""}</div>
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
        backgroundColor: props.background
          ? "rgba(31, 31, 31, 0.92)"
          : undefined,
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
