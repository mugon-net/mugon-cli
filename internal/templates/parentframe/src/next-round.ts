import {
  areSettingsValid,
  type CompiledSettingExpressions,
} from "@mugon/sdk/expression";
import {
  clampNumberSetting,
  defaultSettingValues,
  type SettingDeclaration,
  type SettingValue,
} from "@mugon/sdk/settings";

export type NextRoundResolution =
  | { values: Record<string, SettingValue> }
  | { error: string };

// What the platform does with the settings a game hands to `nextGame`: they
// replace the whole set, settings left out take their defaults, and anything
// the declarations or `settings-valid-when` reject ends the game instead.
export function resolveNextRoundSettings(
  declarations: SettingDeclaration[],
  expressions: CompiledSettingExpressions,
  proposed: Record<string, SettingValue>,
): NextRoundResolution {
  const declared = new Map(
    declarations.map((declaration) => [declaration.key, declaration]),
  );
  for (const key of Object.keys(proposed)) {
    if (!declared.has(key)) {
      return { error: `setting '${key}' is not declared by the game` };
    }
  }

  const values = defaultSettingValues(declarations);
  for (const [key, value] of Object.entries(proposed)) {
    const problem = checkValue(declared.get(key)!, value);
    if (problem) return { error: `setting '${key}': ${problem}` };
    values[key] = value;
  }

  if (!areSettingsValid(expressions, values)) {
    return { error: "the settings do not satisfy settings-valid-when" };
  }
  return { values };
}

function checkValue(
  declaration: SettingDeclaration,
  value: SettingValue,
): string | undefined {
  switch (declaration.type) {
    case "BOOLEAN":
      return typeof value === "boolean" ? undefined : "expected a boolean";
    case "NUMBER":
      if (typeof value !== "number") return "expected a number";
      return clampNumberSetting(declaration, value) === value
        ? undefined
        : "outside the allowed range or step";
    case "SELECT": {
      const allowed = new Set(
        declaration.select?.options.map((option) => option.value),
      );
      if (typeof value !== "string" && !Array.isArray(value)) {
        return "expected a selection";
      }
      const chosen = Array.isArray(value) ? value : [value];
      if (!declaration.select?.multiple && chosen.length > 1) {
        return "expected a single choice";
      }
      return chosen.every((entry) => allowed.has(entry))
        ? undefined
        : "not one of the declared options";
    }
    case "TEXT": {
      if (typeof value !== "string") return "expected text";
      const maxLength = declaration.text?.maxLength;
      return maxLength !== undefined && value.length > maxLength
        ? "longer than the allowed length"
        : undefined;
    }
  }
}
