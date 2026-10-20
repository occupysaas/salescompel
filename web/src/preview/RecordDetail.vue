<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { t } from "../i18n";
import { usePreviewStore } from "../stores/preview";
import { recordLabel, type PreviewObject } from "./model";
import NotFound from "./NotFound.vue";
const props = defineProps<{ object: PreviewObject }>();
const route = useRoute(),
  store = usePreviewStore();
const row = computed(() => store.find(props.object, String(route.params.id)));
const contacts = computed(() =>
  store.contacts.filter((c) => c.AccountId === row.value?.id),
);
const account = computed(() =>
  row.value?.AccountId
    ? store.find("accounts", row.value.AccountId)
    : undefined,
);
</script>
<template>
  <NotFound v-if="!row" record />
  <template v-else>
    <RouterLink :to="`/preview/${object}`"
      >← {{ t("preview.back") }}</RouterLink
    >
    <div class="p-title">
      <div>
        <p class="p-eyebrow">
          {{ t("nav." + object) }} / {{ t("preview.sample") }}
        </p>
        <h1>{{ recordLabel(row) }}</h1>
        <p>{{ t("preview.prototypeFields") }}</p>
      </div>
      <RouterLink class="p-primary" :to="`/preview/${object}/${row.id}/edit`"
        >{{ t("preview.edit") }} <span aria-hidden="true">↗</span></RouterLink
      >
    </div>
    <div class="p-detail-grid">
      <section class="p-panel">
        <h2>
          {{
            t(
              object === "accounts"
                ? "preview.accountDetails"
                : "preview.contactDetails",
            )
          }}
        </h2>
        <dl>
          <template v-if="object === 'accounts'"
            ><dt>{{ t("field.Name") }}</dt>
            <dd>{{ row.Name }}</dd>
            <dt>{{ t("field.Compat_Key__c") }}</dt>
            <dd>
              <code>{{ row.Compat_Key__c || "—" }}</code>
            </dd></template
          ><template v-else
            ><dt>{{ t("field.FirstName") }}</dt>
            <dd>{{ row.FirstName || "—" }}</dd>
            <dt>{{ t("field.LastName") }}</dt>
            <dd>{{ row.LastName }}</dd>
            <dt>{{ t("preview.account") }}</dt>
            <dd>
              <RouterLink
                v-if="account"
                :to="`/preview/accounts/${account.id}`"
                >{{ account.Name }}</RouterLink
              ><span v-else>{{ t("preview.none") }}</span>
            </dd></template
          >
          <dt>{{ t("preview.recordId") }}</dt>
          <dd>
            <code>{{ row.id }}</code>
          </dd>
        </dl>
      </section>
      <section v-if="object === 'accounts'" class="p-panel">
        <h2>{{ t("preview.relatedContacts") }}</h2>
        <p class="p-related-hint">{{ t("preview.relatedHint") }}</p>
        <ul class="p-related-list">
          <li v-for="contact in contacts" :key="contact.id">
            <RouterLink :to="`/preview/contacts/${contact.id}`">{{
              recordLabel(contact)
            }}</RouterLink>
          </li>
        </ul>
        <p v-if="!contacts.length">{{ t("preview.noContacts") }}</p>
      </section>
    </div>
  </template>
</template>
