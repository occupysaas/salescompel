import type { CandidateExchange } from "../../src/candidate-openapi-model.ts";

export const base = "/services/data/v65.0";
export const bytes = (v: unknown) => Buffer.from(JSON.stringify(v));
export const query = (q: string, extra = "") =>
  base + "/query?q=" + encodeURIComponent(q) + extra;
export const exchange = (
  method: string,
  path: string,
  status = 200,
  body: unknown = { done: true, totalSize: 0, records: [] },
): CandidateExchange => ({
  method,
  path,
  request: { contentType: null },
  response: { status, contentType: "application/json", body: bytes(body) },
});

export const parentPath = () =>
  exchange(
    "GET",
    query("SELECT Account.Name FROM Contact", "&columns=true"),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Contact" },
          Account: { attributes: { type: "Account" }, Name: "Acme" },
        },
      ],
      columnMetadata: [
        {
          columnName: "Account",
          joinColumns: [{ columnName: "Name", joinColumns: [] }],
        },
      ],
    },
  );

export const nestedParentPath = () =>
  exchange("GET", query("SELECT Parent__r.Parent__r.Name FROM Child__c"), 200, {
    done: true,
    totalSize: 1,
    records: [
      {
        attributes: { type: "Child__c" },
        Parent__r: {
          attributes: { type: "Parent__c" },
          Parent__r: { attributes: { type: "Parent__c" }, Name: "Root" },
        },
      },
    ],
    columnMetadata: [
      {
        columnName: "Parent__r",
        joinColumns: [
          {
            columnName: "Parent__r",
            joinColumns: [{ columnName: "Name", joinColumns: [] }],
          },
        ],
      },
    ],
  });

export const childRelationship = () =>
  exchange(
    "GET",
    query("SELECT Name, (SELECT LastName FROM Contacts) FROM Account"),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Account" },
          Name: "Acme",
          Contacts: {
            totalSize: 1,
            done: true,
            records: [{ attributes: { type: "Contact" }, LastName: "Child" }],
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        {
          columnName: "Contacts",
          joinColumns: [{ columnName: "LastName", joinColumns: [] }],
        },
      ],
    },
  );

export const childPlainAndParentLeaf = () =>
  exchange(
    "GET",
    query(
      "SELECT Name, (SELECT Name, Account.Name FROM Contacts) FROM Account",
    ),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Account" },
          Name: "Acme",
          Contacts: {
            totalSize: 1,
            done: true,
            records: [
              {
                attributes: { type: "Contact" },
                Name: "Jane Doe",
                Account: { attributes: { type: "Account" }, Name: "Acme" },
              },
            ],
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        {
          columnName: "Contacts",
          joinColumns: [
            { columnName: "Name", joinColumns: [] },
            {
              columnName: "Account",
              joinColumns: [{ columnName: "Name", joinColumns: [] }],
            },
          ],
        },
      ],
    },
  );

export const childParentPath = () =>
  exchange(
    "GET",
    query(
      "SELECT Name, (SELECT Account.Name, MailingAddress FROM Contacts) FROM Account",
    ),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Account" },
          Name: "Acme",
          Contacts: {
            totalSize: 1,
            done: true,
            records: [
              {
                attributes: { type: "Contact" },
                Account: { attributes: { type: "Account" }, Name: "Acme" },
                MailingAddress: {
                  street: "9 Oak",
                  city: "Portland",
                  state: "OR",
                  postalCode: "97201",
                  country: "US",
                },
              },
            ],
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        {
          columnName: "Contacts",
          joinColumns: [
            {
              columnName: "Account",
              joinColumns: [{ columnName: "Name", joinColumns: [] }],
            },
            { columnName: "MailingAddress", joinColumns: [] },
          ],
        },
      ],
    },
  );

export const customParentAccount = () =>
  exchange(
    "GET",
    query("SELECT Name, CustomLookup__r.Name FROM Account"),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Account" },
          Name: "Acme",
          CustomLookup__r: {
            attributes: { type: "Contact" },
            Name: "Linked",
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        {
          columnName: "CustomLookup__r",
          joinColumns: [{ columnName: "Name", joinColumns: [] }],
        },
      ],
    },
  );

export const customParentContact = () =>
  exchange("GET", query("SELECT LastName, Other__r.Name FROM Contact"), 200, {
    done: true,
    totalSize: 1,
    records: [
      {
        attributes: { type: "Contact" },
        LastName: "Child",
        Other__r: { attributes: { type: "Account" }, Name: "Linked" },
      },
    ],
    columnMetadata: [
      { columnName: "LastName" },
      {
        columnName: "Other__r",
        joinColumns: [{ columnName: "Name", joinColumns: [] }],
      },
    ],
  });

export const childDeepParentPath = () =>
  exchange(
    "GET",
    query(
      "SELECT Name, (SELECT A__r.B__r.C__r.D__r.E__r.Name FROM Children__r) FROM Root__c",
    ),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Root__c" },
          Name: "Root",
          Children__r: {
            totalSize: 1,
            done: true,
            records: [
              {
                attributes: { type: "Child__c" },
                A__r: {
                  attributes: { type: "A__c" },
                  B__r: {
                    attributes: { type: "B__c" },
                    C__r: {
                      attributes: { type: "C__c" },
                      D__r: {
                        attributes: { type: "D__c" },
                        E__r: { attributes: { type: "E__c" }, Name: "E" },
                      },
                    },
                  },
                },
              },
            ],
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        {
          columnName: "Children__r",
          joinColumns: [
            {
              columnName: "A__r",
              joinColumns: [
                {
                  columnName: "B__r",
                  joinColumns: [
                    {
                      columnName: "C__r",
                      joinColumns: [
                        {
                          columnName: "D__r",
                          joinColumns: [
                            {
                              columnName: "E__r",
                              joinColumns: [
                                { columnName: "Name", joinColumns: [] },
                              ],
                            },
                          ],
                        },
                      ],
                    },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
  );
