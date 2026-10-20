<script setup lang="ts">
import { computed, ref, onUnmounted } from "vue";
import {
  api,
  setToken,
  type Description,
  type Field,
  type RecordData,
} from "../api";
import { language, languages, setLanguage, t } from "../i18n";
onUnmounted(() => setToken(""));
const draftToken = ref(""),
  connected = ref(false),
  busy = ref(false),
  error = ref("");
const object = ref("Account"),
  description = ref<Description | null>(null),
  records = ref<RecordData[]>([]);
const editing = ref(false),
  recordID = ref(""),
  values = ref<Record<string, string>>({}),
  confirmDelete = ref(false);
const fields = computed(
  () => description.value?.fields.filter((f) => f.name !== "Id") ?? [],
);
const languageNames: Record<string, string> = {
  en_US: "English",
  fr: "Français",
  de: "Deutsch",
  pt_BR: "Português (Brasil)",
  iw: "עברית",
};
function label(field: Field) {
  return t(`field.${field.name}`);
}
function recordName(row: RecordData) {
  return String(
    row[object.value === "Account" ? "Name" : "LastName"] ?? row.Id,
  );
}
async function action(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await fn();
  } catch (e) {
    error.value = e instanceof Error ? e.message : "REQUEST_FAILED";
  } finally {
    busy.value = false;
  }
}
async function load() {
  records.value = [];
  description.value = null;
  const d = await api<Description>(`sobjects/${object.value}/describe`);
  const q = `SELECT ${d.fields.map((f) => f.name).join(",")} FROM ${object.value} ORDER BY ${object.value === "Account" ? "Name" : "LastName"} LIMIT 2000`;
  const result = await api<{ records: RecordData[] }>(
    `query?q=${encodeURIComponent(q)}`,
  );
  description.value = d;
  records.value = result.records;
}
async function connect() {
  await action(async () => {
    setToken(draftToken.value.trim());
    await load();
    connected.value = true;
    draftToken.value = "";
  });
  if (!connected.value) setToken("");
}
function disconnect() {
  setToken("");
  draftToken.value = "";
  connected.value = false;
  records.value = [];
  description.value = null;
  editing.value = false;
  recordID.value = "";
  values.value = {};
  error.value = "";
}
async function switchObject(name: string) {
  await action(async () => {
    object.value = name;
    records.value = [];
    description.value = null;
    editing.value = false;
    await load();
  });
}
function create() {
  recordID.value = "";
  values.value = {};
  editing.value = true;
  confirmDelete.value = false;
  error.value = "";
}
async function edit(row: RecordData) {
  await action(async () => {
    const record = await api<RecordData>(`sobjects/${object.value}/${row.Id}`);
    values.value = {};
    for (const f of fields.value)
      values.value[f.name] = String(record[f.name] ?? "");
    recordID.value = String(row.Id);
    editing.value = true;
    confirmDelete.value = false;
  });
}
async function save() {
  await action(async () => {
    const data: Record<string, string | null> = {};
    for (const f of fields.value) {
      if (recordID.value ? !f.updateable : !f.createable) continue;
      const value = values.value[f.name] ?? "";
      data[f.name] = value === "" && f.nillable ? null : value;
    }
    await api(
      `sobjects/${object.value}${recordID.value ? "/" + recordID.value : ""}`,
      recordID.value ? "PATCH" : "POST",
      data,
    );
    editing.value = false;
    await load();
  });
}
async function remove() {
  await action(async () => {
    await api(`sobjects/${object.value}/${recordID.value}`, "DELETE");
    editing.value = false;
    confirmDelete.value = false;
    await load();
  });
}
async function changeLanguage(event: Event) {
  try {
    await setLanguage((event.target as HTMLSelectElement).value);
  } catch {
    error.value = "LANGUAGE_UNAVAILABLE";
  }
}
</script>

<template>
  <div class="shell">
    <header>
      <a class="brand" href="/"
        >Sales<span>Compel</span><span class="brand-dot">.</span></a
      >
      <div class="header-actions">
        <RouterLink to="/preview">{{ t("preview.open") }}</RouterLink>
        <label class="language"
          >{{ t("language.label")
          }}<select :value="language" @change="changeLanguage">
            <option
              v-for="lang in languages"
              :key="lang.code"
              :value="lang.code"
            >
              {{ languageNames[lang.code] ?? lang.code }}
            </option>
          </select></label
        >
        <button
          v-if="connected"
          class="quiet"
          :disabled="busy"
          @click="disconnect"
        >
          {{ t("auth.disconnect") }}
        </button>
      </div>
    </header>
    <main>
      <p v-if="error" role="alert" class="error">
        {{ t("state.error") }} <code>{{ error }}</code>
      </p>
      <section v-if="!connected" class="welcome">
        <div class="welcome-copy">
          <div class="mark" aria-hidden="true">SC</div>
          <h1>{{ t("app.subtitle") }}</h1>
          <p>{{ t("auth.hint") }}</p>
        </div>
        <form class="login-card" @submit.prevent="connect">
          <label
            >{{ t("auth.token")
            }}<input
              v-model="draftToken"
              type="password"
              autocomplete="off"
              required
              :disabled="busy"
          /></label>
          <button class="primary" :disabled="busy">
            {{ busy ? t("state.loading") : t("auth.connect") }}
          </button>
        </form>
      </section>
      <template v-else>
        <nav :aria-label="t('app.subtitle')">
          <button
            :class="{ active: object === 'Account' }"
            :disabled="busy"
            @click="switchObject('Account')"
          >
            {{ t("nav.accounts") }}</button
          ><button
            :class="{ active: object === 'Contact' }"
            :disabled="busy"
            @click="switchObject('Contact')"
          >
            {{ t("nav.contacts") }}
          </button>
        </nav>
        <div class="section-heading">
          <div>
            <p class="eyebrow">SalesCompel</p>
            <h1>
              {{ t(object === "Account" ? "nav.accounts" : "nav.contacts") }}
            </h1>
          </div>
          <button
            class="primary"
            :disabled="busy || !description"
            @click="create"
          >
            + {{ t("action.new") }}
          </button>
        </div>
        <p v-if="busy" role="status" class="loading">
          {{ t("state.loading") }}
        </p>
        <div
          class="workspace"
          :class="{ 'with-editor': editing }"
          :aria-busy="busy"
        >
          <section class="record-list">
            <p v-if="records.length === 0 && !busy && !error" class="empty">
              {{ t("state.empty") }}
            </p>
            <table v-else-if="records.length">
              <thead>
                <tr>
                  <th>
                    {{
                      t(object === "Account" ? "field.Name" : "field.LastName")
                    }}
                  </th>
                  <th>
                    {{
                      t(
                        object === "Account"
                          ? "field.Compat_Key__c"
                          : "field.AccountId",
                      )
                    }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in records"
                  :key="String(row.Id)"
                  :class="{ selected: recordID === row.Id && editing }"
                >
                  <td>
                    <button
                      class="record-link"
                      :disabled="busy"
                      @click="edit(row)"
                    >
                      {{ recordName(row) }}
                    </button>
                  </td>
                  <td class="secondary">
                    {{
                      row[
                        object === "Account" ? "Compat_Key__c" : "AccountId"
                      ] ?? "—"
                    }}
                  </td>
                </tr>
              </tbody>
            </table>
          </section>
          <section v-if="editing" class="editor">
            <h2>{{ t(recordID ? "editor.edit" : "editor.new") }}</h2>
            <p v-if="recordID" data-testid="record-id" class="record-id">
              {{ recordID }}
            </p>
            <form @submit.prevent="save">
              <fieldset :disabled="busy">
                <label v-for="field in fields" :key="field.name"
                  >{{ label(field)
                  }}<input
                    v-model="values[field.name]"
                    :required="!field.nillable"
                    :maxlength="field.length || undefined"
                    :disabled="
                      recordID ? !field.updateable : !field.createable
                    "
                /></label>
                <div class="form-actions">
                  <button class="primary" type="submit">
                    {{ t("action.save") }}</button
                  ><button type="button" @click="editing = false">
                    {{ t("action.cancel") }}
                  </button>
                </div>
                <button
                  v-if="recordID && !confirmDelete"
                  class="danger"
                  type="button"
                  @click="confirmDelete = true"
                >
                  {{ t("action.delete") }}
                </button>
                <div v-if="confirmDelete" class="confirmation">
                  <p>{{ t("delete.question") }}</p>
                  <button class="danger" type="button" @click="remove">
                    {{ t("action.confirm") }}</button
                  ><button type="button" @click="confirmDelete = false">
                    {{ t("action.cancel") }}
                  </button>
                </div>
              </fieldset>
            </form>
          </section>
        </div>
      </template>
    </main>
  </div>
</template>
