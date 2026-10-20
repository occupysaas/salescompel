<script setup lang="ts">
import { t } from "../i18n";
defineProps<{ record?: boolean }>();
</script>
<template>
  <section class="p-panel">
    <p class="p-eyebrow">404</p>
    <h1>{{ t(record ? "preview.notFound" : "preview.pageNotFound") }}</h1>
    <p>{{ t("preview.notFoundHint") }}</p>
    <RouterLink to="/preview/accounts">{{ t("nav.accounts") }} →</RouterLink>
  </section>
</template>
