export interface Language {
  code: string;
  bcp47: string;
  direction: string;
  fallback: string | null;
}
export function resolveLanguage(languages: Language[], code: string): Language {
  const lang =
    languages.find((l) => l.code === code) ??
    languages.find((l) => l.code === "en_US");
  if (!lang) throw new Error("English fallback missing");
  return lang;
}
export function translate(
  messages: Record<string, string>,
  fallback: Record<string, string>,
  key: string,
): string {
  return messages[key] ?? fallback[key] ?? key;
}
