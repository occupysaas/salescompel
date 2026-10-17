# Reference UI navigation inventory

Manual black-box observation of the reference product's standard navigation, made
by the maintainer while browsing the authorized disposable reference org. It records
**what navigation exists and how it is organized**, not how it looks.

| Field | Value |
| --- | --- |
| Captured | 2026-10-17 (single manual pass, not repeated) |
| Method | Manual browsing as administrator; no automation, screenshots or DOM capture |
| Environment | Authorized reference tenant recorded in `../implementation-log.md` |
| Release / edition / licenses | Not recorded — the visible apps depend on enabled licenses and features |
| Status | Unreviewed field notes; not a contract or acceptance evidence |

## Policy fit

Under the [black-box testing policy](../../../CLEAN_ROOM.md), observing a public UI
and describing its externally visible behavior independently is permitted. This
file is a functional inventory: app names, navigation items and default list-view
names are largely the names of standard objects and list views that imported
metadata and migrated users already rely on, so they are compatibility identifiers.

It is **not** a design source. Do not copy the reference UI's templates, styles,
icons, images, layout or help prose into SalesCompel. The Vue UI is designed
independently; this inventory only tells us which destinations users expect to
find and how customizable the navigation must be.

## Observed navigation model

Interpretation of the notes below (observations first, then inference):

- **Observed:** a side menu lists top-level areas; selecting one shows a horizontal
  menu of items for that area. Many items expand to show sub-entries.
- **Observed:** the same item (for example Contacts) appears in several areas and
  shows the same sub-entries everywhere it appears.
- **Observed:** the horizontal menus are configurable — any item from the
  [available item list](#available-navigation-items) can be added to any area.
- **Documented** (see below): top-level areas are *apps*, horizontal items are
  *navigation items*, and sub-entries are the item menu's list views, recent records
  and "New" actions.
- **Inference:** these correspond to `CustomApplication`, `CustomTab` and `ListView`
  metadata. "Recently Viewed" appears to be a built-in list rather than a stored
  list view. Confirm by retrieving the metadata before relying on this.
- **Inference:** because navigation is item-based and reusable, SalesCompel should
  implement a metadata-driven app/navigation model that can import these
  definitions, rather than hard-coding a fixed menu.

## What the documentation says

Summarized independently from public documentation read on 2026-10-17. The
`sources.yaml` IDs are given in brackets. Secondary or third-party sources are marked;
re-check them before they inform a contract.

**How navigation works** [`lightning-navigation`]

- An *app* is a collection of items for a particular kind of work. The navigation
  bar (our "horizontal menu") shows the app's name, optional branding and the items
  the admin assigned to it. Items that don't fit go under *More*.
- The *App Launcher* lists every app the user can open (*All Apps*) and every item
  they have permission to use (*All Items*), with search across both.
- *Navigation items* are standard and custom objects plus other pages such as
  reports, dashboards and Chatter. Object items have an *item menu* (our "expander")
  with shortcuts: create a new record, recent records and list views. The menu
  differs by item; for example Tasks and Notes show no recent records, while Reports
  and Dashboards show only recent records.
- *List views* are saved, filtered views of records. "All Contacts", "My Contacts",
  "Today's Leads" and similar entries are list views, which explains why the same
  sub-menu appears wherever an item is placed: it belongs to the object, not the app.
- Users can personalize an app's bar: add objects, list views, records or pages,
  reorder by dragging and remove their own additions. They cannot rename or remove
  admin-defined items. Each app's bar holds at most 50 items. *Favorites* (up to
  200) are separate personal shortcuts; an object's three most-used favorites also
  appear in its item menu.

**Which product this org appears to be** [`starter-suite-overview`]

Starter Suite (and the higher Pro Suite tier) is Salesforce's small-business bundle
combining sales, service, marketing and commerce in one org. Its documented areas
match the observed top-level areas, so the reference org is probably one of these
suites. *Inference:* confirm the edition from Company Information. A full Enterprise
org organizes apps differently, so this inventory isn't the general Salesforce layout.
Pro Suite adds custom apps and objects, personalized navigation and layouts, flow
automation, quotes and forecasts [`starter-pro-comparison`, third-party].

| Area | Documented purpose |
| --- | --- |
| Home | Start-of-day page linking to the main areas, with recommended action cards. A *Guidance Center* panel in the header covers setup tasks: import data, invite users, create first records [`starter-suite-overview`]. |
| Sales | Leads and opportunities to track deals, sales paths (stage guidance) and revenue reports [`starter-suite-overview`]. |
| Service | Cases and a knowledge base for customer support, with AI help drafting replies and summarizing interactions [`starter-suite-overview`]. |
| Marketing | Lists and segments to target audiences, email templates and automated sends, branded landing pages and lead-capture forms. Sending promotional email requires recorded consent for every recipient [`starter-suite-overview`, `starter-email-marketing`]. |
| Products (Commerce) | *Commerce Storefronts*: a direct-to-consumer online store with a setup assistant, product import, a store template, and workspaces for products, pricing, customers, buyer groups, promotions and orders [`starter-storefronts`]. *Pay Now* stores send payment links that customers pay on a hosted page; links can be one-off or reusable and attached to any record. They require a Salesforce Payments license [`pay-now`, `paymentlink-object`]. |
| Automation | The *Automation* app (introduced Summer '24) lets flow admins create, open and monitor flows. Its list views show recently modified flows and flows with errors. It is enabled in Process Automation Settings [secondary; no official page found yet]. |
| Account | Probably the vendor's in-app subscription and billing area ("Your Account"). Not a CRM function. |

Item-level notes: *Actionable Lists* are lists built from data-platform segments for
sales follow-up, with per-prospect status tracking [`actionable-lists`]. The search
found no documentation for Marketing *Briefs* or *List Sends*. Quick Text, Messaging
Sessions, Knowledge and Price Books are standard objects with their own reference
pages; look them up when they come into scope.

## Top-level areas and their items

Labels are recorded as seen; a few obvious typos in the field notes were corrected.
"(expands)" means the item opened a sub-menu.

1. **Home**
2. **Contacts** (expands): All Contacts · Recently Viewed · My Contacts · New Contacts · Birthdays
3. **Accounts** (expands): All Accounts · Recently Viewed · My Accounts · New Accounts
4. **Sales** (expands)
   - Leads (expands): All Open Leads · My Unread Leads · Today's Leads
   - Contacts (expands): same entries as area 2
   - Accounts (expands): All Accounts · My Accounts · New Accounts
   - Opportunities (expands): All Opportunities · Opportunity Pipelines · My Opportunities · New Opportunities · Won
   - Products (expands): All Products
   - Price Books (expands): New Price Book
   - Calendar (expands): My Team's Recent Events · My Recent Events · My Team's Upcoming Events · My Upcoming Events · Today's Agenda
   - Analytics
   - Invoices (expands)
   - Video Calls (expands)
5. **Service** (expands)
   - Cases (expands)
   - Contacts (expands): same entries as area 2
   - Accounts (expands): All Accounts · My Accounts · New Accounts
   - Quick Text (expands)
   - Messaging Sessions (expands)
   - Analytics
   - Knowledge (expands): New Knowledge · Published Articles · Draft Articles
6. **Marketing** (expands)
   - Home
   - Campaigns (expands): New Campaigns · Campaign List
   - Contacts (expands): same entries as area 2
   - Leads (expands): All Open Leads · My Unread Leads · Today's Leads
   - Actionable Lists (expands): New Actionable List
   - Segments (expands)
   - Flows (expands)
   - Content
   - Analytics
   - Briefs (expands): New Brief
   - Consent
   - List Sends
7. **Products** (expands) — commerce storefronts, with a store selector; multiple stores are allowed.
   - Standard store:
     - Home (expands)
     - Merchandising (expands): Products · Categories · Pricing · Synonym Rules · Product Curation
     - Customers (expands): Accounts · Buyer Groups
     - Promotions
     - Orders
     - Insights
     - Settings (expands): Store · Checkout · Product · Search · Messaging · Domains
     - Website Design
     - Experience Builder
     - Content Manager
   - "Pay now" store (multiple allowed):
     - Home
     - Merchandising (collapsible): Product Curation
     - Payment Links
     - Settings (expands): Store · Checkout · Domains
     - Experience Builder
     - Content Manager
8. **Account** — vendor subscription/billing area; probably not needed for a self-hosted free product.
9. **Automation** (expands): Home · Flows · Integrations · Monitor

Open questions for a later pass: the sub-entries of items that expanded but were not
recorded (Invoices, Video Calls, Cases, Quick Text, Messaging Sessions, Segments,
Flows), and which areas/items depend on specific licenses.

## Available navigation items

Items offered when customizing an area's horizontal menu, as listed (alphabetical).
Many are standard objects; some are tools or setup surfaces. Items with the same
name carry the same sub-menu wherever they are placed.

Account Brands · Accounts · Action Hub · Actionable Lists · Agent Config ·
Alternative Payment Methods · Analytics · App Launcher · Approval Requests · Assets ·
Authorization Form · Authorization Form Consent · Authorization Form Data Use ·
Authorization Form Text · Background Operations · Business Brands · Buyer Criteria ·
Buyer Group Price Books · Buyer Group Related Objects · Buyer Groups ·
Calculated Affinities · Calculated Insight Range Bounds · Calendar · Campaigns ·
Card Payment Methods · Carts · Cases · Catalogs · Categories · Chat Sessions ·
Chat Transcripts · Chat Visitors · Communication Subscription Channel Types ·
Communication Subscription Consents · Communication Subscription Timings ·
Communication Subscriptions · Contact Point Consent · Contact Point Type Consent ·
Contact Requests · Contacts · Content · Content Taxonomy · Conversation Insights ·
Coupons · Credit Memos · Customer Workspace · Customers · Dashboards ·
Data Use Legal Basis · Data Use Purpose · Datasets · Digital Wallets ·
Duplicate Record Sets · Email Content · Email Templates · Engagement Channel Types ·
Engagement Signals · Entitlement Policies · Experiments · Explore Cards · Files ·
Flow Versions · Flows · Fulfillment Orders · Geo Countries · Geo States ·
Getting Started · Global Flow Resources · Guest Buyer Profiles · Home · Images ·
Individuals · Integrations · Invoices · Knowledge · Labels · Leads · Legal Entities ·
List Emails · Locations · Marketing · Messaging Sessions · Messaging Users · Monitor ·
Notebook AI · Notes · Opportunities · Order Delivery Methods ·
Order Payment Summaries · Order Payment Summary References · Order Summaries ·
Order Summary Additional Infos · Order Summary Relationships ·
Order Summary Routing Schedules · Org Metrics · Partner Offer Items · Partner Offers ·
Party Consent · Paused Flows · Payment Authorization Adjustments ·
Payment Authorizations · Payment Gateway Logs · Payment Gateways ·
Payment Line Invoices · Payments · Price Adjustment Schedules · Price Books ·
Pricing Workspace · Privacy Preference Manager · Product Relationship Types ·
Product Workspace · Products · Promotion Segments · Promotion Workspace · Promotions ·
Quick Text · Record Collections · Recycle Bin · Refund Line Payments · Refunds ·
Regrello · Reports · Sales Channels · Segments · Sellers · Shipments · Shortcuts ·
Store Price Books · Tasks · Tax Engines · Tax Geo Configurations · Tax Policies ·
Tax Rates · Tax Treatment Items · Tax Treatments · User Provisioning Requests ·
Video Call Participants · Video Calls · Voice Calls · Work Plan Templates ·
Work Plans · Work Step Templates · Your Account

## Next steps

- Retrieve `CustomApplication`, `CustomTab` and `ListView` metadata from the reference
  org through the Metadata API to confirm the inferences above with documented
  structure, and record the release and enabled licenses.
- Use this inventory to scope the M2/M6 Vue navigation: which apps and items the
  Sales Cloud pilot needs first (Home, Leads, Accounts, Contacts, Opportunities,
  Products, Price Books, Calendar, Campaigns, Cases, Reports, Dashboards) versus
  later clouds (Service, Marketing, Commerce).
- If navigation behavior becomes an acceptance target, turn it into repeatable UI
  scenarios (fixed role, configuration and viewport) as described in the roadmap.
