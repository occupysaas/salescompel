import type { WireResponse } from "./wire.ts";

// These checks cover a documented subset, not a complete compatibility verdict.
export function validateLimitsResponse(response: WireResponse): void {
  if (response.status !== 200 || !isRecord(response.body))
    throw new Error("Limits response was not a successful JSON object");
  const daily = response.body.DailyApiRequests;
  if (!isRecord(daily) || !isCounter(daily.Max) || !isCounter(daily.Remaining))
    throw new Error("Limits response lacks valid DailyApiRequests counters");
}

export function validateLimitInfo(value: string | undefined): void {
  if (
    value === undefined ||
    !value.split(";").some((entry) => /^api-usage=\d+\/\d+$/.test(entry.trim()))
  )
    throw new Error("Limit info lacks a valid api-usage entry");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isCounter(value: unknown): boolean {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}
