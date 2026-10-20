import { ref } from "vue";
import { createI18n } from "vue-i18n";
import registry from "../../locales/languages.json";
import { resolveLanguage, latestLocaleLoader } from "./i18n-core";
const catalogs = import.meta.glob<Record<string, string>>("../../locales/messages/*.json", { import: "default" });
export const languages = registry.languages;
export const language = ref("en_US");
export const i18n = createI18n({ legacy: false, locale: "en_US", fallbackLocale: "en_US", flatJson: true, missingWarn: false, fallbackWarn: false, messages: {} });
export const setLanguage = latestLocaleLoader(async (code: string) => {
  const selected = resolveLanguage(languages, code);
  const [english, chosen] = await Promise.all([
    catalogs["../../locales/messages/en_US.json"]!(),
    catalogs[`../../locales/messages/${selected.code}.json`]!(),
  ]);
  return { selected, english, chosen };
}, ({ selected, english, chosen }) => {
  i18n.global.setLocaleMessage("en_US", english);
  i18n.global.setLocaleMessage(selected.code, chosen);
  i18n.global.locale.value = selected.code;
  language.value = selected.code;
  document.documentElement.lang = selected.bcp47;
  document.documentElement.dir = selected.direction;
});
export function t(key: string): string { return i18n.global.t(key); }
