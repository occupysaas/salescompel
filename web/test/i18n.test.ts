import test from "node:test";
import assert from "node:assert/strict";
import { resolveLanguage, translate } from "../src/i18n-core.ts";
test("language selection preserves aliases, variants and fallback", () => {
  const langs = [
    { code: "en_US", bcp47: "en-US", direction: "ltr", fallback: null },
    { code: "iw", bcp47: "he", direction: "rtl", fallback: "en_US" },
    { code: "pt_BR", bcp47: "pt-BR", direction: "ltr", fallback: "en_US" },
  ];
  assert.equal(resolveLanguage(langs, "iw").direction, "rtl");
  assert.equal(resolveLanguage(langs, "pt_BR").bcp47, "pt-BR");
  assert.equal(resolveLanguage(langs, "unknown").code, "en_US");
  assert.equal(
    translate({ save: "Salvar" }, { save: "Save", cancel: "Cancel" }, "save"),
    "Salvar",
  );
  assert.equal(translate({}, { cancel: "Cancel" }, "cancel"), "Cancel");
});
