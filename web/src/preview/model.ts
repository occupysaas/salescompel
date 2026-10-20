export type PreviewObject = "accounts" | "contacts";
export interface PreviewRecord {
  id: string;
  Name?: string;
  FirstName?: string;
  LastName?: string;
  AccountId?: string | null;
  Compat_Key__c?: string | null;
}
export function recordLabel(record: PreviewRecord): string {
  return (
    record.Name ?? [record.FirstName, record.LastName].filter(Boolean).join(" ")
  );
}
