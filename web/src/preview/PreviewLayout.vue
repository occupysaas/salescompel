<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { t, languages } from "../i18n";
import { usePreferencesStore, type Theme } from "../stores/preferences";
import { usePreviewStore } from "../stores/preview";
import "./preview.css";
import PreviewNavItem from "./PreviewNavItem.vue";
import { navigation } from "./navigation";
const preferences = usePreferencesStore(),
  preview = usePreviewStore();
const route = useRoute(),
  router = useRouter();
const open = ref(false),
  menuButton = ref<HTMLButtonElement>();
const main = ref<HTMLElement>();
const loadError = ref("");
const names: Record<string, string> = {
  en_US: "English",
  fr: "Français",
  de: "Deutsch",
  pt_BR: "Português (Brasil)",
  iw: "עברית",
};
async function close() {
  open.value = false;
  await nextTick();
  menuButton.value?.focus();
}
watch(
  () => route.fullPath,
  async () => {
    const wasOpen = open.value;
    open.value = false;
    await nextTick();
    if (wasOpen) menuButton.value?.focus();
    else main.value?.focus();
  },
);
async function chooseLocale(event: Event) {
  try {
    await preferences.setLocale((event.target as HTMLSelectElement).value);
    loadError.value = "";
  } catch {
    loadError.value = "preview.languageError";
  }
}
async function reset() {
  preview.reset();
  await router.push("/preview/accounts");
}
</script>
<template>
  <div class="preview-app" @keydown.esc="open && close()">
    <a class="p-skip" href="#preview-main" @click.prevent="main?.focus()">{{
      t("preview.skip")
    }}</a>
    <header class="p-header">
      <RouterLink class="p-brand" to="/preview/accounts"
        >SALES<span>COMPEL</span><span class="p-brand-dot">.</span></RouterLink
      >
      <span class="p-badge">{{ t("preview.badge") }}</span>
      <div class="p-preferences">
        <label
          ><span aria-hidden="true">◎ </span>{{ t("language.label")
          }}<select :value="preferences.locale" @change="chooseLocale">
            <option
              v-for="lang in languages"
              :key="lang.code"
              :value="lang.code"
            >
              {{ names[lang.code] ?? lang.code }}
            </option>
          </select></label
        >
        <label
          >{{ t("preview.theme")
          }}<select
            :value="preferences.theme"
            @change="
              preferences.setTheme(
                ($event.target as HTMLSelectElement).value as Theme,
              )
            "
          >
            <option value="system">{{ t("preview.system") }}</option>
            <option value="light">{{ t("preview.light") }}</option>
            <option value="dark">{{ t("preview.dark") }}</option>
          </select></label
        >
      </div>
      <button
        ref="menuButton"
        class="p-menu"
        :aria-expanded="open"
        aria-controls="preview-nav"
        @click="open = !open"
      >
        {{ t(open ? "preview.closeNav" : "preview.openNav") }}
      </button>
    </header>
    <div class="p-frame">
      <aside id="preview-nav" class="p-sidebar" :class="{ 'p-open': open }">
        <div class="p-nav-label">{{ t("preview.workspace") }}</div>
        <nav :aria-label="t('preview.navigation')">
          <ul class="p-nav-items">
            <PreviewNavItem
              v-for="item in navigation"
              :key="item.key"
              :item="item"
            />
          </ul>
        </nav>
        <div class="p-sidebar-bottom">
          <p>{{ t("preview.sampleWorkspace") }}</p>
          <button @click="reset">{{ t("preview.reset") }}</button
          ><RouterLink to="/">{{ t("preview.return") }}</RouterLink>
        </div>
      </aside>
      <main id="preview-main" ref="main" class="p-main" tabindex="-1">
        <div class="p-notice">
          <span class="p-notice-label">{{ t("preview.noticeLabel") }}</span>
          <p>{{ t("preview.notice") }}</p>
        </div>
        <p v-if="loadError" role="alert" class="p-error">{{ t(loadError) }}</p>
        <p class="p-announcement" role="status" aria-live="polite">
          {{ preview.announcement ? t(preview.announcement) : "" }}
        </p>
        <RouterView :key="route.path" />
        <footer class="p-footer">
          <span>{{ t("preview.footer") }}</span
          ><RouterLink to="/preview/evidence"
            >{{ t("preview.viewEvidence") }} →</RouterLink
          >
        </footer>
      </main>
    </div>
  </div>
</template>
