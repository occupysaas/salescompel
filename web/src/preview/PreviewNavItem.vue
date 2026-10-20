<script setup lang="ts">
import { ref, useId } from "vue";
import { useI18n } from "vue-i18n";
import type { PreviewNavigationItem } from "./navigation";
defineProps<{ item: PreviewNavigationItem }>();
const { t } = useI18n({ useScope: "global" });
const expanded = ref(false);
const submenuId = `preview-submenu-${useId()}`;
</script>
<template>
  <li>
    <div class="p-nav-row">
      <RouterLink v-if="item.path" :to="item.path">{{
        t(item.label)
      }}</RouterLink>
      <button v-else-if="!item.children" class="p-nav-planned" disabled>
        <span>{{ t(item.label) }}</span
        ><span class="p-planned-tag">{{ t("preview.planned") }}</span>
      </button>
      <button
        v-if="item.children"
        class="p-nav-expand"
        :class="{ 'p-nav-group': !item.path }"
        :aria-label="t('preview.itemNavigation', { item: t(item.label) })"
        :aria-expanded="expanded"
        :aria-controls="submenuId"
        @click="expanded = !expanded"
      >
        <span v-if="!item.path">{{ t(item.label) }}</span
        ><span
          class="p-chevron"
          :class="{ 'p-expanded': expanded }"
          aria-hidden="true"
          >›</span
        >
      </button>
    </div>
    <ul
      v-if="item.children"
      v-show="expanded"
      :id="submenuId"
      role="group"
      :aria-label="t(item.label)"
      class="p-nav-children"
    >
      <PreviewNavItem
        v-for="child in item.children"
        :key="child.key"
        :item="child"
      />
    </ul>
  </li>
</template>
