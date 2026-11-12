<script setup lang="ts">
import { computed, ref, onUnmounted } from "vue";
import {
  loadCatalog,
  loadCurrency,
  loadStoredFields,
  type ObjectSummary,
  type StoredField,
} from "./catalog";
import FieldInput from "./FieldInput.vue";
import {
  formatValue,
  fromApiValue,
  toApiValue,
  type ControlValue,
} from "./fields";
import {
  api as request,
  setToken,
  type Description,
  type Field,
  type RecordData,
} from "../api";
import { language, languages, setLanguage, t } from "../i18n";
const lifetime = new AbortController();
const objects = ref<ObjectSummary[]>([]);
let generation = 0;
onUnmounted(() => {
  lifetime.abort();
  setToken("");
});
async function api<T>(
  path: string,
  method = "GET",
  data?: unknown,
): Promise<T> {
  if (lifetime.signal.aborted)
    throw new DOMException("Route left", "AbortError");
  return request<T>(path, method, data, lifetime.signal);
}
const draftToken = ref(""),
  connected = ref(false),
  busy = ref(false),
  error = ref("");
const object = ref("Account"),
  description = ref<Description | null>(null),
  records = ref<RecordData[]>([]);
const editing = ref(false),
  recordID = ref(""),
  values = ref<Record<string, ControlValue>>({}),
  current = ref<RecordData>({}),
  confirmDelete = ref(false);
const currency = ref("USD"),
  stored = ref<StoredField[]>([]),
  lookupOptions = ref<Record<string, { id: string; name: string }[]>>({});
const locale = computed(
  () => languages.find((l) => l.code === language.value)?.bcp47 ?? "en-US",
);
function writable(field: Field) {
  return recordID.value ? field.updateable : field.createable;
}
async function loadLookups(d: Description, current: number) {
  const out: Record<string, { id: string; name: string }[]> = {};
  for (const f of d.fields) {
    const target = f.referenceTo?.[0];
    if (f.type !== "reference" || !target || out[f.name]) continue;
    try {
      const q = `SELECT Id,Name FROM ${target} ORDER BY Name LIMIT 200`;
      const r = await api<{ records: { Id: string; Name: unknown }[] }>(
        `query?q=${encodeURIComponent(q)}`,
      );
      out[f.name] = r.records.map((x) => ({
        id: x.Id,
        name: String(x.Name ?? x.Id),
      }));
    } catch {
      out[f.name] = []; // manual ID entry remains available
    }
  }
  if (current === generation) lookupOptions.value = out;
}
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
  const seeded: Record<string, readonly string[]> = {
    Account: ["Id", "Name", "Compat_Text__c", "Compat_Key__c"],
    Contact: ["Id", "FirstName", "LastName", "AccountId"],
  };
  return seeded[object.value]?.includes(field.name)
    ? t(`field.${field.name}`)
    : field.label || field.name;
}
const nameFieldLabel = computed(() => {
  const name = object.value === "Contact" ? "LastName" : "Name";
  const f = fields.value.find((f) => f.name === name);
  return f ? label(f) : name;
});
const secondaryField = computed(() =>
  fields.value.find(
    (f) =>
      f.name ===
      (object.value === "Account"
        ? "Compat_Key__c"
        : object.value === "Contact"
          ? "AccountId"
          : ""),
  ),
);
function objectLabel(item?: ObjectSummary) {
  if (!item) return object.value;
  if (item.name === "Account") return t("nav.accounts");
  if (item.name === "Contact") return t("nav.contacts");
  return item.labelPlural || item.label || item.name;
}
async function refreshCatalog() {
  await action(async () => {
    generation++;
    editing.value = false;
    records.value = [];
    description.value = null;
    objects.value = await loadCatalog(lifetime.signal);
    currency.value = await loadCurrency(lifetime.signal);
    if (!objects.value.some((o) => o.name === object.value))
      object.value = objects.value[0]?.name ?? "";
    if (object.value) await load();
  });
}
function recordName(row: RecordData) {
  return String(
    row[object.value === "Contact" ? "LastName" : "Name"] ?? row.Id,
  );
}
async function action(fn: () => Promise<void>) {
  if (busy.value || lifetime.signal.aborted) return;
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
  const current = ++generation;
  records.value = [];
  description.value = null;
  editing.value = false;
  const d = await api<Description>(`sobjects/${object.value}/describe`);
  const storedFields = await loadStoredFields(object.value, lifetime.signal);
  const q = `SELECT ${d.fields.map((f) => f.name).join(",")} FROM ${object.value} ORDER BY ${object.value === "Contact" ? "LastName" : "Name"} LIMIT 2000`;
  const result = await api<{ records: RecordData[] }>(
    `query?q=${encodeURIComponent(q)}`,
  );
  if (current !== generation || lifetime.signal.aborted) return;
  description.value = d;
  records.value = result.records;
  stored.value = storedFields;
  await loadLookups(d, current);
}
async function connect() {
  await action(async () => {
    setToken(draftToken.value.trim());
    objects.value = await loadCatalog(lifetime.signal);
    currency.value = await loadCurrency(lifetime.signal);
    object.value = objects.value.some((o) => o.name === "Account")
      ? "Account"
      : (objects.value[0]?.name ?? "");
    if (!object.value) throw new Error("INVALID_TYPE");
    await load();
    connected.value = true;
    draftToken.value = "";
  });
  if (!lifetime.signal.aborted && !connected.value) setToken("");
}
function disconnect() {
  generation++;
  objects.value = [];
  setToken("");
  draftToken.value = "";
  connected.value = false;
  records.value = [];
  description.value = null;
  editing.value = false;
  recordID.value = "";
  values.value = {};
  current.value = {};
  stored.value = [];
  lookupOptions.value = {};
  currency.value = "USD";
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
  current.value = {};
  values.value = {};
  for (const f of fields.value) {
    const def = f.picklistValues?.find((p) => p.defaultValue && p.active);
    values.value[f.name] = fromApiValue(
      f,
      f.type === "boolean" ? f.defaultValue === true : (def?.value ?? null),
    );
  }
  editing.value = true;
  confirmDelete.value = false;
  error.value = "";
}
async function edit(row: RecordData) {
  await action(async () => {
    const record = await api<RecordData>(`sobjects/${object.value}/${row.Id}`);
    values.value = {};
    for (const f of fields.value)
      values.value[f.name] = fromApiValue(f, record[f.name]);
    current.value = record;
    recordID.value = String(row.Id);
    editing.value = true;
    confirmDelete.value = false;
  });
}
async function save() {
  await action(async () => {
    const data: Record<string, unknown> = {};
    for (const f of fields.value) {
      if (!writable(f)) continue;
      data[f.name] = toApiValue(f, values.value[f.name] ?? "");
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
            v-for="item in objects"
            :key="item.name"
            :class="{ active: object === item.name }"
            :disabled="busy"
            @click="switchObject(item.name)"
          >
            {{ objectLabel(item) }}
          </button>
          <button class="quiet" :disabled="busy" @click="refreshCatalog">
            {{ t("action.refreshObjects") }}
          </button>
        </nav>
        <div class="section-heading">
          <div>
            <p class="eyebrow">SalesCompel</p>
            <h1>
              {{ objectLabel(objects.find((o) => o.name === object)) }}
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
                  <th scope="col">{{ nameFieldLabel }}</th>
                  <th v-if="secondaryField" scope="col">
                    {{ label(secondaryField) }}
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
                  <td v-if="secondaryField" class="secondary">
                    {{
                      formatValue(
                        secondaryField,
                        row[secondaryField.name],
                        locale,
                        currency,
                      )
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
                <template v-for="field in fields" :key="field.name">
                  <FieldInput
                    v-if="writable(field) || recordID"
                    v-model="values[field.name]"
                    :field="field"
                    :label="label(field)"
                    :disabled="busy"
                    :readonly="!writable(field)"
                    :display="current[field.name]"
                    :locale="locale"
                    :currency="currency"
                    :lookup-options="lookupOptions[field.name] ?? []"
                  />
                </template>
                <div class="form-actions">
                  <button class="primary" type="submit">
                    {{ t("action.save") }}</button><button
                    type="button"
                    @click="editing = false"
                  >
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
                    {{ t("action.confirm") }}</button><button
                    type="button"
                    @click="confirmDelete = false"
                  >
                    {{ t("action.cancel") }}
                  </button>
                </div>
              </fieldset>
            </form>
          </section>
        </div>
        <section v-if="stored.length" class="stored-note" role="note">
          <h2>{{ t("field.storedHeading") }}</h2>
          <p>{{ t("field.storedHint") }}</p>
          <ul>
            <li v-for="item in stored" :key="item.name">
              {{ item.label }} <code>{{ item.name }}</code> — {{ item.type }}
            </li>
          </ul>
        </section>
      </template>
    </main>
  </div>
</template>
