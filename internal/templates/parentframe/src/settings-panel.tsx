import type { CompiledSettingExpressions } from "@mugon/sdk/expression";
import { isSettingEnabled } from "@mugon/sdk/expression";
import { SettingDeclaration, SettingValue } from "@mugon/sdk/settings";

export function SettingsPanel({
  declarations,
  values,
  expressions,
  onChange,
}: {
  declarations: SettingDeclaration[];
  values: Record<string, SettingValue>;
  expressions: CompiledSettingExpressions;
  onChange: (key: string, value: SettingValue) => void;
}) {
  if (declarations.length === 0) return null;

  return (
    <div style={panelStyle}>
      <div style={{ color: "white", fontWeight: 600 }}>Game settings</div>
      <div style={{ color: "#AAAAAA", fontSize: "0.8rem" }}>
        Only used when you start as host. A client joins with whatever the host
        set up and ignores these.
      </div>
      {declarations.map((declaration) => (
        <label key={declaration.key} style={rowStyle}>
          <span style={{ color: "#DDDDDD" }} title={declaration.description}>
            {declaration.label}
          </span>
          <SettingInput
            declaration={declaration}
            value={values[declaration.key]}
            disabled={!isSettingEnabled(expressions, declaration.key, values)}
            onChange={(value) => onChange(declaration.key, value)}
          />
        </label>
      ))}
    </div>
  );
}

function SettingInput({
  declaration,
  value,
  disabled,
  onChange,
}: {
  declaration: SettingDeclaration;
  value: SettingValue | undefined;
  disabled: boolean;
  onChange: (value: SettingValue) => void;
}) {
  switch (declaration.type) {
    case "BOOLEAN":
      return (
        <input
          type="checkbox"
          disabled={disabled}
          checked={value === true}
          onChange={(event) => onChange(event.target.checked)}
        />
      );
    case "NUMBER":
      return (
        <input
          type="number"
          disabled={disabled}
          min={declaration.number?.min}
          max={declaration.number?.max}
          step={declaration.number?.step}
          value={typeof value === "number" ? value : 0}
          onChange={(event) => onChange(Number(event.target.value))}
          style={inputStyle}
        />
      );
    case "SELECT":
      return (
        <select
          disabled={disabled}
          multiple={declaration.select?.multiple}
          value={value as string | string[]}
          onChange={(event) =>
            onChange(
              declaration.select?.multiple
                ? Array.from(event.target.selectedOptions, (o) => o.value)
                : event.target.value,
            )
          }
          style={inputStyle}
        >
          {declaration.select?.options.map((settingOption) => (
            <option key={settingOption.value} value={settingOption.value}>
              {settingOption.label}
            </option>
          ))}
        </select>
      );
    case "TEXT":
      return (
        <input
          type="text"
          disabled={disabled}
          maxLength={declaration.text?.maxLength}
          value={typeof value === "string" ? value : ""}
          onChange={(event) => onChange(event.target.value)}
          style={inputStyle}
        />
      );
  }
}

const panelStyle: React.CSSProperties = {
  display: "flex",
  flexDirection: "column",
  gap: "0.5rem",
  padding: "1rem",
  width: "22rem",
  maxWidth: "90%",
  maxHeight: "50vh",
  overflowY: "auto",
  textAlign: "left",
  boxSizing: "border-box",
  backgroundColor: "#2E2E2E",
  border: "1px solid #3F3F3F",
  borderRadius: "8px",
};

const rowStyle: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  gap: "1rem",
  fontSize: "0.9rem",
};

const inputStyle: React.CSSProperties = {
  backgroundColor: "#1F1F1F",
  color: "white",
  border: "1px solid #3F3F3F",
  borderRadius: "4px",
  padding: "0.25rem 0.5rem",
};
