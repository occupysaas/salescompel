export interface PreferenceReader { getItem(key: string): string | null }
export interface PreferenceWriter { setItem(key: string, value: string): void }
export function readPreference(storage: PreferenceReader | undefined, key: string, allowed: readonly string[], fallback: string): string {
  try { const value = storage?.getItem(key); return value && allowed.includes(value) ? value : fallback; } catch { return fallback; }
}
export function writePreference(storage: PreferenceWriter | undefined, key: string, value: string): void {
  try { storage?.setItem(key, value); } catch { /* Preferences are optional. */ }
}
export function browserStorage(): Storage | undefined {
  try { return window.localStorage; } catch { return undefined; }
}
