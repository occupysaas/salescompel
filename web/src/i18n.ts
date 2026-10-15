import { ref } from "vue";
import registry from "../../locales/languages.json";
import { resolveLanguage, translate } from "./i18n-core";
const catalogs = import.meta.glob<Record<string, string>>(
  "../../locales/messages/*.json",
  { import: "default" },
);
export const languages = registry.languages;
export const language = ref("en_US");
const messages = ref<Record<string, string>>({});
let fallback: Record<string, string> = {};
let sequence = 0;
export async function setLanguage(code: string) {
  const ticket = ++sequence;
  const selected = resolveLanguage(languages, code);
  const [english, chosen] = await Promise.all([
    catalogs["../../locales/messages/en_US.json"]!(),
    catalogs[`../../locales/messages/${selected.code}.json`]!(),
  ]);
  if (ticket !== sequence) return;
  fallback = english;
  messages.value = chosen;
  language.value = selected.code;
  document.documentElement.lang = selected.bcp47;
  document.documentElement.dir = selected.direction;
}
export function t(key: string) {
  return translate(messages.value, fallback, key);
}
