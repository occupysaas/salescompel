import { api } from "../api";
export interface ObjectSummary {
  name: string;
  label: string;
  labelPlural: string;
  createable: boolean;
  queryable: boolean;
}
export async function loadCatalog(
  signal: AbortSignal,
): Promise<ObjectSummary[]> {
  const result = await api<{ sobjects: ObjectSummary[] }>(
    "sobjects",
    "GET",
    undefined,
    signal,
  );
  return result.sobjects.filter((o) => o.queryable);
}
export interface StoredField {
  object: string;
  name: string;
  label: string;
  type: string;
}
/** Stored-not-enforced definitions retained for an object (candidate-only). */
export async function loadStoredFields(
  object: string,
  signal: AbortSignal,
): Promise<StoredField[]> {
  const result = await api<{ storedFields: StoredField[] }>(
    `candidate/sobjects/${encodeURIComponent(object)}/storedFields`,
    "GET",
    undefined,
    signal,
  );
  return result.storedFields;
}
/** Tenant corporate currency, used only for display formatting. */
export async function loadCurrency(signal: AbortSignal): Promise<string> {
  const q = "SELECT DefaultCurrencyIsoCode FROM Organization";
  const result = await api<{
    records: { DefaultCurrencyIsoCode: string }[];
  }>(`query?q=${encodeURIComponent(q)}`, "GET", undefined, signal);
  return result.records[0]?.DefaultCurrencyIsoCode ?? "USD";
}
