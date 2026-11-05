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
