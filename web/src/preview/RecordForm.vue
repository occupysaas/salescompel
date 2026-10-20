<script setup lang="ts">
import { computed, reactive, ref, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { t } from "../i18n";
import { usePreviewStore } from "../stores/preview";
import type { PreviewObject } from "./model";
import NotFound from "./NotFound.vue";
const props = defineProps<{ object: PreviewObject }>();
const route = useRoute(),
  router = useRouter(),
  store = usePreviewStore();
const id = typeof route.params.id === "string" ? route.params.id : undefined;
const existing = id ? store.find(props.object, id) : undefined;
const missing = computed(() => !!id && !store.find(props.object, id));
const draft = reactive({
  Name: existing?.Name ?? "",
  FirstName: existing?.FirstName ?? "",
  LastName: existing?.LastName ?? "",
  AccountId: existing?.AccountId ?? "",
  Compat_Key__c: existing?.Compat_Key__c ?? "",
});
const error = ref(""),
  alert = ref<HTMLParagraphElement>();
async function save() {
  try {
    const data =
      props.object === "accounts"
        ? { Name: draft.Name, Compat_Key__c: draft.Compat_Key__c || null }
        : {
            FirstName: draft.FirstName,
            LastName: draft.LastName,
            AccountId: draft.AccountId || null,
          };
    const saved = store.save(props.object, data, id);
    await router.push(`/preview/${props.object}/${saved}`);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "preview.required";
    await nextTick();
    alert.value?.focus();
  }
}
</script>
<template>
  <NotFound v-if="missing" record />
  <template v-else>
    <RouterLink :to="`/preview/${object}${id ? '/' + id : ''}`"
      >← {{ t("preview.back") }}</RouterLink
    >
    <div class="p-title">
      <div>
        <p class="p-eyebrow">
          {{ t("nav." + object) }} / {{ t("preview.forms") }}
        </p>
        <h1>
          {{
            t(
              id
                ? "preview.edit"
                : object === "accounts"
                  ? "preview.newAccount"
                  : "preview.newContact",
            )
          }}
        </h1>
        <p>{{ t(id ? "preview.editHint" : "preview.newHint") }}</p>
      </div>
    </div>
    <form class="p-panel p-form" novalidate @submit.prevent="save">
      <p class="p-related-hint">{{ t("preview.formRequired") }}</p>
      <p v-if="error" ref="alert" class="p-error" role="alert" tabindex="-1">
        {{ t(error) }}
      </p>
      <template v-if="object === 'accounts'">
        <div class="p-field">
          <label id="sample-name-label" for="sample-name"
            >{{ t("field.Name") }}<span aria-hidden="true"> *</span></label
          ><input
            id="sample-name"
            v-model="draft.Name"
            :aria-label="t('field.Name')"
            aria-required="true"
            :aria-invalid="error === 'preview.required'"
            autocomplete="off"
          />
        </div>
        <div class="p-field">
          <label for="sample-key">{{ t("field.Compat_Key__c") }}</label
          ><input
            id="sample-key"
            v-model="draft.Compat_Key__c"
            :aria-invalid="error === 'preview.keyLength'"
            autocomplete="off"
          />
        </div>
      </template>
      <template v-else>
        <div class="p-field">
          <label for="sample-first">{{ t("field.FirstName") }}</label
          ><input
            id="sample-first"
            v-model="draft.FirstName"
            autocomplete="off"
          />
        </div>
        <div class="p-field">
          <label for="sample-last"
            >{{ t("field.LastName") }}<span aria-hidden="true"> *</span></label
          ><input
            id="sample-last"
            v-model="draft.LastName"
            :aria-label="t('field.LastName')"
            aria-required="true"
            :aria-invalid="error === 'preview.required'"
            autocomplete="off"
          />
        </div>
        <div class="p-field">
          <label for="sample-account">{{ t("preview.account") }}</label
          ><select id="sample-account" v-model="draft.AccountId">
            <option value="">{{ t("preview.none") }}</option>
            <option
              v-for="account in store.accounts"
              :key="account.id"
              :value="account.id"
            >
              {{ account.Name }}
            </option>
          </select>
        </div>
      </template>
      <div class="p-actions">
        <button class="p-primary" type="submit">{{ t("preview.save") }}</button
        ><RouterLink :to="`/preview/${object}${id ? '/' + id : ''}`">{{
          t("action.cancel")
        }}</RouterLink>
      </div>
    </form>
  </template>
</template>
