import test from "node:test";
import assert from "node:assert/strict";
import {
  RawNumber,
  serializeBody,
  parseBody,
  validateDecimal,
  formatValue,
  toApiValue,
  fromApiValue,
  changedFields,
  type FieldInfo,
} from "../src/connected/fields.ts";

const f = (type: string, extra: Partial<FieldInfo> = {}): FieldInfo => ({
  name: "X__c",
  type,
  nillable: true,
  createable: true,
  updateable: true,
  length: 0,
  precision: 18,
  scale: 2,
  ...extra,
});

test("decimal validation uses digit strings, never Number()", () => {
  assert.equal(validateDecimal("1234567890123456.79", 18, 2), "ok");
  assert.equal(validateDecimal("12345678901234567.89", 18, 2), "range");
  assert.equal(validateDecimal("-0.5", 18, 2), "ok");
  assert.equal(validateDecimal("1e3", 18, 2), "syntax");
  assert.equal(validateDecimal("1,5", 18, 2), "syntax");
  assert.equal(validateDecimal("", 18, 2), "syntax");
  assert.equal(validateDecimal("1000", 5, 2), "range");
  assert.equal(validateDecimal("999.99", 5, 2), "ok");
  assert.equal(validateDecimal("0.123", 5, 2), "ok");
});

test("raw numbers keep their exact text in requests and responses", () => {
  const body = serializeBody({
    Num__c: toApiValue(f("double"), "1234567890123456.79"),
    T__c: "1234567890123456.79",
  });
  assert.equal(
    body,
    '{"Num__c":1234567890123456.79,"T__c":"1234567890123456.79"}',
  );
  const parsed = parseBody('{"a":1234567890123456.79,"b":12.5,"c":"x"}') as any;
  assert.ok(parsed.a instanceof RawNumber);
  assert.equal(parsed.a.text, "1234567890123456.79");
  assert.equal(parsed.b, 12.5);
  assert.equal(fromApiValue(f("currency"), parsed.a), "1234567890123456.79");
});

test("values convert to the API form for each type", () => {
  assert.equal(toApiValue(f("currency"), ""), null);
  assert.equal(toApiValue(f("boolean"), true), true);
  assert.equal(toApiValue(f("multipicklist"), ["A", "C"]), "A;C");
  assert.equal(toApiValue(f("multipicklist"), []), null);
  assert.equal(toApiValue(f("date"), "2026-11-11"), "2026-11-11");
  assert.equal(toApiValue(f("reference"), ""), null);
  assert.equal(toApiValue(f("string", { nillable: false }), ""), "");
  const local = toApiValue(f("datetime"), "2026-03-09T04:30") as string;
  assert.equal(local, new Date("2026-03-09T04:30").toISOString());
  assert.deepEqual(fromApiValue(f("multipicklist"), "A;C"), ["A", "C"]);
  assert.equal(fromApiValue(f("boolean"), false), false);
  assert.equal(fromApiValue(f("string"), null), "");
});

test("display formatting follows locale and tenant currency", () => {
  assert.equal(formatValue(f("currency"), 1234.5, "en-US", "USD"), "$1,234.50");
  assert.equal(
    formatValue(f("currency"), 1234.5, "de-DE", "EUR").replace(/\s/g, " "),
    "1.234,50 €",
  );
  assert.equal(
    formatValue(f("percent", { scale: 1 }), 12.5, "en-US", "USD"),
    "12.5%",
  );
  assert.equal(formatValue(f("multipicklist"), "A;C", "en-US", "USD"), "A, C");
  assert.equal(formatValue(f("boolean"), true, "en-US", "USD"), "✓");
  assert.equal(formatValue(f("double"), null, "en-US", "USD"), "—");
  assert.equal(
    formatValue(
      f("address"),
      {
        street: null,
        city: "Lyon",
        state: null,
        postalCode: null,
        country: "France",
      },
      "en-US",
      "USD",
    ),
    "Lyon, France",
  );
  assert.equal(
    formatValue(
      f("double", { scale: 2 }),
      new RawNumber("1234567890123456.79"),
      "en-US",
      "USD",
    ),
    "1,234,567,890,123,456.79",
  );
  assert.equal(
    formatValue(f("url"), "javascript:alert(1)", "en-US", "USD"),
    "javascript:alert(1)",
  );
});

test("updates send only the fields the user changed", () => {
  const fields = [
    f("datetime", { name: "At__c" }),
    f("string", { name: "Name" }),
    f("multipicklist", { name: "M__c" }),
  ];
  const original = { At__c: "2026-03-09T04:30", Name: "A", M__c: ["A"] };
  assert.deepEqual(
    changedFields(fields, original, { ...original, Name: "B" }),
    ["Name"],
  );
  assert.deepEqual(
    changedFields(fields, original, { ...original, M__c: ["A", "B"] }),
    ["M__c"],
  );
  assert.deepEqual(changedFields(fields, original, { ...original }), []);
});
