<script setup lang="ts">
import { computed } from "vue";
import { t } from "../i18n";
import { usePreviewStore } from "../stores/preview";
import { recordLabel, type PreviewObject } from "./model";
const props = defineProps<{ object: PreviewObject }>();
const store = usePreviewStore();
const rows = computed(() =>
  props.object === "accounts" ? store.accounts : store.contacts,
);
</script>
<template>
  <div class="p-title">
    <div>
      <p class="p-eyebrow">
        {{ t("preview.workspace") }} / {{ t("preview.records") }}
      </p>
      <h1>{{ t("nav." + object) }}</h1>
      <p>
        {{
          t(
            object === "accounts"
              ? "preview.listIntroAccounts"
              : "preview.listIntroContacts",
          )
        }}
      </p>
    </div>
    <RouterLink class="p-primary" :to="`/preview/${object}/new`"
      ><span aria-hidden="true">＋ </span
      >{{
        t(object === "accounts" ? "preview.newAccount" : "preview.newContact")
      }}</RouterLink
    >
  </div>
  <div class="p-table-wrap">
    <table>
      <thead>
        <tr>
          <th v-if="object === 'accounts'" scope="col">
            {{ t("field.Name") }}
          </th>
          <template v-else
            ><th scope="col">{{ t("field.FirstName") }}</th>
            <th scope="col">{{ t("field.LastName") }}</th></template
          >
          <th scope="col">
            {{
              t(
                object === "accounts"
                  ? "field.Compat_Key__c"
                  : "preview.account",
              )
            }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.id">
          <td v-if="object === 'accounts'">
            <RouterLink :to="`/preview/${object}/${row.id}`">{{
              recordLabel(row)
            }}</RouterLink
            ><span class="p-secondary"
              >{{ t("preview.sample") }} · {{ row.id }}</span
            >
          </td>
          <template v-else
            ><td>{{ row.FirstName || "—" }}</td>
            <td>
              <RouterLink :to="`/preview/${object}/${row.id}`">{{
                row.LastName
              }}</RouterLink
              ><span class="p-secondary">{{ row.id }}</span>
            </td></template
          >
          <td v-if="object === 'accounts'">
            <code>{{ row.Compat_Key__c || "—" }}</code>
          </td>
          <td v-else>
            <RouterLink
              v-if="row.AccountId && store.find('accounts', row.AccountId)"
              :to="`/preview/accounts/${row.AccountId}`"
              >{{ store.find("accounts", row.AccountId)?.Name }}</RouterLink
            ><span v-else>{{ t("preview.none") }}</span>
          </td>
        </tr>
        <tr v-if="!rows.length">
          <td :colspan="object === 'accounts' ? 2 : 3">
            {{ t("preview.empty") }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <div class="p-count">
    <span>{{ rows.length }} {{ t("preview.sampleCount") }}</span
    ><span>{{ t("preview.badge") }}</span>
  </div>
</template>
