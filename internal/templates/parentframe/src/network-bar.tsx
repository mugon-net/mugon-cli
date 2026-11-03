import {
  MAX_MS,
  MAX_PERCENT,
  NETWORK_PRESETS,
  NetworkProfile,
  sameNetworkProfile,
} from "./network-profile";

function clamp(value: number, max: number): number {
  if (Number.isNaN(value)) return 0;
  return Math.min(max, Math.max(0, value));
}

export function NetworkBar({
  profile,
  onChange,
}: {
  profile: NetworkProfile;
  onChange: (profile: NetworkProfile) => void;
}) {
  return (
    <div style={barStyle}>
      <span style={{ color: "white", fontWeight: 600 }}>Network</span>
      {NETWORK_PRESETS.map((preset) => (
        <button
          key={preset.label}
          className="button"
          style={{
            ...presetStyle,
            borderColor: sameNetworkProfile(profile, preset.profile)
              ? "#7BD88F"
              : undefined,
          }}
          onClick={() => onChange(preset.profile)}
        >
          {preset.label}
        </button>
      ))}
      <NumberField
        label="Latency (ms, one way)"
        value={profile.latencyMs}
        max={MAX_MS}
        step={5}
        onChange={(latencyMs) => onChange({ ...profile, latencyMs })}
      />
      <NumberField
        label="Jitter (ms)"
        value={profile.jitterMs}
        max={MAX_MS}
        step={5}
        onChange={(jitterMs) => onChange({ ...profile, jitterMs })}
      />
      <NumberField
        label="Loss (%)"
        value={profile.lossPercent}
        max={MAX_PERCENT}
        step={0.1}
        onChange={(lossPercent) => onChange({ ...profile, lossPercent })}
      />
    </div>
  );
}

function NumberField({
  label,
  value,
  max,
  step,
  onChange,
}: {
  label: string;
  value: number;
  max: number;
  step: number;
  onChange: (value: number) => void;
}) {
  return (
    <label style={{ display: "flex", alignItems: "center", gap: "0.4rem" }}>
      <span>{label}</span>
      <input
        type="number"
        min={0}
        max={max}
        step={step}
        value={value}
        style={{ width: "5rem" }}
        onChange={(event) => onChange(clamp(event.target.valueAsNumber, max))}
      />
    </label>
  );
}

const barStyle: React.CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  alignItems: "center",
  justifyContent: "center",
  gap: "0.75rem",
  color: "#DDDDDD",
  fontSize: "0.85rem",
};

const presetStyle: React.CSSProperties = {
  padding: "0.3rem 0.8rem",
  fontSize: "0.85rem",
};
