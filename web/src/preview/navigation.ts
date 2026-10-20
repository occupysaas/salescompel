export interface PreviewNavigationItem {
  key: string;
  label: string;
  path?: string;
  children?: PreviewNavigationItem[];
}
const planned = (key: string): PreviewNavigationItem => ({
  key,
  label: `preview.nav.${key}`,
});
const contacts = (): PreviewNavigationItem => ({
  key: "contacts",
  label: "nav.contacts",
  path: "/preview/contacts",
  children: [
    {
      key: "allContacts",
      label: "preview.nav.allContacts",
      path: "/preview/contacts",
    },
    ...["recentlyViewed", "myContacts", "newContacts", "birthdays"].map(
      planned,
    ),
  ],
});
const accounts = (): PreviewNavigationItem => ({
  key: "accounts",
  label: "nav.accounts",
  path: "/preview/accounts",
  children: [
    {
      key: "allAccounts",
      label: "preview.nav.allAccounts",
      path: "/preview/accounts",
    },
    ...["recentlyViewed", "myAccounts", "newAccounts"].map(planned),
  ],
});
// Functional organization from the maintainer's reference navigation inventory.
// A destination without a path or supported children is planned and inert.
export const navigation: PreviewNavigationItem[] = [
  planned("home"),
  contacts(),
  accounts(),
  {
    key: "sales",
    label: "preview.nav.sales",
    children: [
      planned("leads"),
      contacts(),
      accounts(),
      ...[
        "opportunities",
        "products",
        "priceBooks",
        "calendar",
        "analytics",
        "invoices",
        "videoCalls",
      ].map(planned),
    ],
  },
  {
    key: "service",
    label: "preview.nav.service",
    children: [
      planned("cases"),
      contacts(),
      accounts(),
      ...["quickText", "messagingSessions", "analytics", "knowledge"].map(
        planned,
      ),
    ],
  },
  {
    key: "marketing",
    label: "preview.nav.marketing",
    children: [
      planned("home"),
      planned("campaigns"),
      contacts(),
      ...[
        "leads",
        "actionableLists",
        "segments",
        "flows",
        "content",
        "analytics",
        "briefs",
        "consent",
        "listSends",
      ].map(planned),
    ],
  },
  planned("products"),
  planned("automation"),
];
