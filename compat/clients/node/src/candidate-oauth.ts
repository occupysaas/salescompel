/** Treat the browser launcher file as untrusted; never browse another origin. */
export function parseLauncherURL(text: string, candidateURL: string): string {
  try {
    const raw = text.trim();
    if (!raw || /\s/.test(raw)) throw new Error();
    const url = new URL(raw),
      candidate = new URL(candidateURL);
    if (
      url.origin !== candidate.origin ||
      url.username ||
      url.password ||
      url.hash ||
      url.pathname !== "/services/oauth2/authorize"
    )
      throw new Error();
    return url.href;
  } catch {
    throw new Error("RESULT");
  }
}

/** Called before output reaches an evidence file, including encoded form values. */
export function redactOAuthEvidence(
  text: string,
  secrets: readonly string[],
): string {
  const variants = new Set<string>();
  for (const secret of secrets) {
    if (!secret) continue;
    variants.add(secret);
    variants.add(encodeURIComponent(secret));
    variants.add(new URLSearchParams({ v: secret }).toString().slice(2));
    variants.add(JSON.stringify(secret).slice(1, -1));
  }
  for (const variant of [...variants])
    variants.add(variant.replace(/%[0-9A-F]{2}/g, (s) => s.toLowerCase()));
  for (const secret of [...variants].sort((a, b) => b.length - a.length))
    text = text.split(secret).join("[REDACTED]");
  return text;
}
