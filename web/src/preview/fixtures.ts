import type { PreviewRecord } from "./model";
export function previewFixtures(): {
  accounts: PreviewRecord[];
  contacts: PreviewRecord[];
} {
  return {
    accounts: [
      {
        id: "sample-account-1",
        Name: "Northstar Studio",
        Compat_Key__c: "sample-northstar",
      },
      {
        id: "sample-account-2",
        Name: "Fieldwork Collective",
        Compat_Key__c: "sample-fieldwork",
      },
    ],
    contacts: [
      {
        id: "sample-contact-1",
        FirstName: "Alex",
        LastName: "Rivera",
        AccountId: "sample-account-1",
      },
      {
        id: "sample-contact-2",
        FirstName: "Morgan",
        LastName: "Chen",
        AccountId: "sample-account-2",
      },
    ],
  };
}
