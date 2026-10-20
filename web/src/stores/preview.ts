import { defineStore } from "pinia";
import { ref } from "vue";
import { previewFixtures } from "../preview/fixtures.ts";
import type { PreviewObject, PreviewRecord } from "../preview/model";
export const usePreviewStore = defineStore("preview", () => {
  const seed = previewFixtures();
  const accounts = ref(seed.accounts),
    contacts = ref(seed.contacts);
  const announcement = ref("");
  let counter = 0;
  function find(object: PreviewObject, id: string): PreviewRecord | undefined {
    return (object === "accounts" ? accounts.value : contacts.value).find(
      (r) => r.id === id,
    );
  }
  function save(
    object: PreviewObject,
    draft: Omit<PreviewRecord, "id">,
    id?: string,
  ): string {
    const list = object === "accounts" ? accounts.value : contacts.value;
    const index = id ? list.findIndex((r) => r.id === id) : -1;
    if (id && index < 0) throw Error("preview.notFound");
    if (!(object === "accounts" ? draft.Name : draft.LastName)?.trim())
      throw Error("preview.required");
    if ((draft.Compat_Key__c?.length ?? 0) > 80)
      throw Error("preview.keyLength");
    if (
      object === "contacts" &&
      draft.AccountId &&
      !find("accounts", draft.AccountId)
    )
      throw Error("preview.missingAccount");
    const record = { ...draft, id: id ?? `sample-local-${++counter}` };
    if (index < 0) list.push(record);
    else list[index] = record;
    announcement.value = "preview.saved";
    return record.id;
  }
  function reset() {
    const next = previewFixtures();
    accounts.value = next.accounts;
    contacts.value = next.contacts;
    announcement.value = "preview.resetDone";
  }
  return { accounts, contacts, announcement, find, save, reset };
});
