export interface WireResponse {
  status: number;
  headers: Record<string, string>;
  body: unknown;
  bodyPresent?: boolean;
}
// The response is private evidence. Never publish message text from this value.
export async function wire(
  base: URL,
  token: string,
  path: string,
  init: RequestInit,
  limits = { maxBytes: 8 * 1024 * 1024, timeoutMs: 15000 },
): Promise<WireResponse> {
  const url = new URL(path, base);
  if (
    !path.startsWith("/services/") ||
    path.includes("\\") ||
    url.origin !== base.origin ||
    url.username ||
    url.password ||
    url.hash ||
    !url.pathname.startsWith("/services/")
  )
    throw new Error(
      "Wire request must use a same-origin relative service path",
    );
  if (
    !Number.isSafeInteger(limits.maxBytes) ||
    limits.maxBytes < 1 ||
    !Number.isSafeInteger(limits.timeoutMs) ||
    limits.timeoutMs < 1
  )
    throw new Error("Invalid wire resource limits");
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${token}`);
  headers.set("Accept", "application/json");
  if (init.body !== undefined && !headers.has("Content-Type"))
    headers.set("Content-Type", "application/json");
  const timeout = AbortSignal.timeout(limits.timeoutMs);
  const response = await fetch(url, {
    ...init,
    headers,
    redirect: "error",
    signal: init.signal ? AbortSignal.any([init.signal, timeout]) : timeout,
  });
  const reader = response.body?.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    if (reader)
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        length += value.byteLength;
        if (length > limits.maxBytes)
          throw new Error("Wire response exceeds size bound");
        chunks.push(value);
      }
  } finally {
    await reader?.cancel().catch(() => {});
  }
  const raw = Buffer.concat(chunks).toString("utf8");
  let body: unknown = null;
  if (raw) {
    try {
      body = JSON.parse(raw);
    } catch {
      body = { nonJson: true, bytes: length };
    }
  }
  const selected: Record<string, string> = {};
  for (const key of ["content-type", "sforce-limit-info", "allow"]) {
    const value = response.headers.get(key);
    if (value !== null) selected[key] = value;
  }
  return {
    status: response.status,
    headers: selected,
    body,
    bodyPresent: raw.length > 0,
  };
}
