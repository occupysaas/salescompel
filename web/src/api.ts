export interface Field {
  name: string;
  type: string;
  nillable: boolean;
  createable: boolean;
  updateable: boolean;
  length: number;
}
export interface Description {
  name: string;
  fields: Field[];
}
export type RecordData = Record<string, string | null | object>;
let token = "";
export function setToken(value: string) {
  token = value;
}
export async function api<T>(
  path: string,
  method = "GET",
  data?: unknown,
): Promise<T> {
  const response = await fetch(`/services/data/v65.0/${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(data ? { "Content-Type": "application/json" } : {}),
    },
    body: data ? JSON.stringify(data) : undefined,
  });
  if (!response.ok) {
    let code = "REQUEST_FAILED";
    try {
      const errors = await response.json();
      code = errors[0]?.errorCode ?? code;
    } catch {}
    throw new Error(code);
  }
  return response.status === 204 ? (undefined as T) : response.json();
}
