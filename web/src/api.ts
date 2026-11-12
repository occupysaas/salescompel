import { parseBody, serializeBody } from "./connected/fields";
export interface PicklistValue {
  active: boolean;
  defaultValue: boolean;
  label: string;
  value: string;
}
export interface Field {
  name: string;
  label?: string;
  type: string;
  nillable: boolean;
  createable: boolean;
  updateable: boolean;
  length: number;
  precision?: number;
  scale?: number;
  digits?: number;
  picklistValues?: PicklistValue[];
  restrictedPicklist?: boolean;
  referenceTo?: string[];
  autoNumber?: boolean;
  compoundFieldName?: string | null;
  defaultValue?: unknown;
  nameField?: boolean;
}
export interface Description {
  name: string;
  fields: Field[];
}
export type RecordData = Record<string, unknown>;
let token = "";
export function setToken(value: string) {
  token = value;
}
export async function api<T>(
  path: string,
  method = "GET",
  data?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const response = await fetch(`/services/data/v65.0/${path}`, {
    method,
    signal,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(data ? { "Content-Type": "application/json" } : {}),
    },
    body: data ? serializeBody(data) : undefined,
  });
  if (!response.ok) {
    let code = "REQUEST_FAILED";
    try {
      const errors = await response.json();
      code = errors[0]?.errorCode ?? code;
    } catch {}
    throw new Error(code);
  }
  return response.status === 204
    ? (undefined as T)
    : (parseBody(await response.text()) as T);
}
