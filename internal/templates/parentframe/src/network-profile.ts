// Simulated network condition of every client link; shared with the Go relay
// (internal/relay/netsim.go) — keep the field names and ranges in sync.
export interface NetworkProfile {
  latencyMs: number;
  jitterMs: number;
  lossPercent: number;
}

export const MAX_MS = 5000;
export const MAX_PERCENT = 100;

export const NETWORK_OFF: NetworkProfile = {
  latencyMs: 0,
  jitterMs: 0,
  lossPercent: 0,
};

export const NETWORK_PRESETS: { label: string; profile: NetworkProfile }[] = [
  { label: "Off", profile: NETWORK_OFF },
  {
    label: "Good Wi-Fi",
    profile: { latencyMs: 15, jitterMs: 5, lossPercent: 0.1 },
  },
  { label: "4G", profile: { latencyMs: 50, jitterMs: 20, lossPercent: 1 } },
  { label: "Poor", profile: { latencyMs: 150, jitterMs: 50, lossPercent: 5 } },
  {
    label: "Awful",
    profile: { latencyMs: 300, jitterMs: 100, lossPercent: 15 },
  },
];

export function isNetworkOff(profile: NetworkProfile): boolean {
  return (
    profile.latencyMs === 0 &&
    profile.jitterMs === 0 &&
    profile.lossPercent === 0
  );
}

export function sameNetworkProfile(
  left: NetworkProfile,
  right: NetworkProfile,
): boolean {
  return (
    left.latencyMs === right.latencyMs &&
    left.jitterMs === right.jitterMs &&
    left.lossPercent === right.lossPercent
  );
}
