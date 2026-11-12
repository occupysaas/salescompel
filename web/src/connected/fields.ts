// Field value rules for the connected editor. Numeric values travel as exact
// decimal text: they are never converted with Number() for validation or
// requests. This module has no imports so unit tests run it directly.

export interface FieldInfo {
  name: string;
  type: string;
  nillable: boolean;
  createable: boolean;
  updateable: boolean;
  length: number;
  precision?: number;
  scale?: number;
  digits?: number;
}

/** An exact JSON number literal, kept as text. */
export class RawNumber {
  readonly text: string;
  constructor(text: string) {
    this.text = text;
  }
  toString() {
    return this.text;
  }
}

const numericTypes = new Set(["double", "int", "currency", "percent"]);
export const isNumeric = (f: FieldInfo) => numericTypes.has(f.type);

/** Precision and scale for a numeric field (int fields use digits, scale 0). */
export function bounds(f: FieldInfo): [number, number] {
  return f.type === "int"
    ? [f.digits ?? 18, 0]
    : [f.precision ?? 18, f.scale ?? 0];
}

/** Validate decimal text against precision/scale by digit counting. Extra
 * fractional digits are allowed: the server rounds them half-up. */
export function validateDecimal(
  text: string,
  precision: number,
  scale: number,
): "ok" | "syntax" | "range" {
  const m = /^-?(\d*)(?:\.(\d*))?$/.exec(text.trim());
  if (!m || (m[1] === "" && (m[2] ?? "") === "")) return "syntax";
  const whole = m[1].replace(/^0+/, "");
  return whole.length > precision - scale ? "range" : "ok";
}

/** JSON.stringify that writes RawNumber values as unquoted number literals. */
export function serializeBody(data: unknown): string {
  const raw: string[] = [];
  const json = JSON.stringify(data, (_key, value) => {
    if (value instanceof RawNumber) {
      raw.push(value.text);
      return `\u0000raw${raw.length - 1}\u0000`;
    }
    return value;
  });
  return json.replace(/"\\u0000raw(\d+)\\u0000"/g, (_m, i) => raw[Number(i)]);
}

/** JSON.parse that keeps numbers whose text a double cannot represent exactly. */
export function parseBody(text: string): unknown {
  return JSON.parse(
    text,
    function (_key, value, context?: { source?: string }) {
      if (
        typeof value === "number" &&
        context?.source &&
        String(value) !== context.source
      )
        return new RawNumber(context.source);
      return value;
    },
  );
}

export type ControlValue = string | boolean | string[];

/** Convert an API value to the editor control's value. */
export function fromApiValue(f: FieldInfo, raw: unknown): ControlValue {
  if (f.type === "boolean") return raw === true;
  if (f.type === "multipicklist")
    return typeof raw === "string" && raw ? raw.split(";") : [];
  if (raw === null || raw === undefined) return "";
  if (f.type === "datetime") {
    const d = new Date(String(raw).replace(/\+0000$/, "Z"));
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
  return String(raw);
}

/** Convert an editor control value to the request value. */
export function toApiValue(f: FieldInfo, input: ControlValue): unknown {
  if (f.type === "boolean") return input === true;
  if (f.type === "multipicklist")
    return Array.isArray(input) && input.length ? input.join(";") : null;
  const text = String(input ?? "");
  if (text === "")
    return f.nillable ||
      isNumeric(f) ||
      f.type === "date" ||
      f.type === "datetime" ||
      f.type === "reference"
      ? null
      : "";
  if (isNumeric(f)) return new RawNumber(text.trim());
  if (f.type === "datetime") return new Date(text).toISOString();
  return text;
}

/** Display text for lists and read-only fields. Tenant text is never HTML. */
export function formatValue(
  f: FieldInfo,
  value: unknown,
  locale: string,
  currency: string,
): string {
  if (value === null || value === undefined || value === "") return "—";
  if (isNumeric(f)) {
    const [, scale] = bounds(f);
    const text = value instanceof RawNumber ? value.text : String(value);
    const options: Intl.NumberFormatOptions =
      f.type === "currency"
        ? {
            style: "currency",
            currency,
            minimumFractionDigits: scale,
            maximumFractionDigits: scale,
          }
        : {
            minimumFractionDigits: 0,
            maximumFractionDigits: Math.max(scale, 0),
          };
    // Intl formats decimal strings exactly (no binary rounding).
    const formatted = new Intl.NumberFormat(locale, options).format(
      text as unknown as number,
    );
    return f.type === "percent" ? formatted + "%" : formatted;
  }
  switch (f.type) {
    case "boolean":
      return value === true ? "✓" : "";
    case "multipicklist":
      return String(value).split(";").join(", ");
    case "date":
      return new Intl.DateTimeFormat(locale, {
        timeZone: "UTC",
        dateStyle: "medium",
      }).format(new Date(String(value) + "T00:00:00Z"));
    case "datetime":
      return new Intl.DateTimeFormat(locale, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(String(value).replace(/\+0000$/, "Z")));
    case "address": {
      const a = value as Record<string, string | null>;
      return ["street", "city", "state", "postalCode", "country"]
        .map((k) => a[k])
        .filter(Boolean)
        .join(", ");
    }
  }
  return String(value);
}
