import { defineStore } from "pinia";
import { ref } from "vue";
import { language, languages, setLanguage } from "../i18n";
import {
  browserStorage,
  readPreference,
  writePreference,
} from "../preferences-core";
export type Theme = "system" | "light" | "dark";
export const usePreferencesStore = defineStore("preferences", () => {
  const theme = ref<Theme>("system");
  const locale = language;
  function setTheme(value: Theme) {
    theme.value = value;
    if (value === "system") delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = value;
    writePreference(browserStorage(), "salescompel.theme", value);
  }
  async function setLocale(code: string) {
    await setLanguage(code);
    writePreference(browserStorage(), "salescompel.locale", locale.value);
  }
  async function initialize() {
    setTheme(
      readPreference(
        browserStorage(),
        "salescompel.theme",
        ["system", "light", "dark"],
        "system",
      ) as Theme,
    );
    await setLocale(
      readPreference(
        browserStorage(),
        "salescompel.locale",
        languages.map((l) => l.code),
        "en_US",
      ),
    );
  }
  return { theme, locale, setTheme, setLocale, initialize };
});
