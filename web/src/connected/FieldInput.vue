<script setup lang="ts">
import { computed } from "vue";
import type { Field } from "../api";
import {
  bounds,
  formatValue,
  isNumeric,
  validateDecimal,
  type ControlValue,
} from "./fields";
import { t } from "../i18n";

const props = defineProps<{
  field: Field;
  label: string;
  modelValue: ControlValue;
  disabled: boolean;
  readonly: boolean;
  display: unknown;
  locale: string;
  currency: string;
  lookupOptions: { id: string; name: string }[];
}>();
const emit = defineEmits<{ "update:modelValue": [value: ControlValue] }>();

const text = computed({
  get: () => (typeof props.modelValue === "string" ? props.modelValue : ""),
  set: (v: string) => emit("update:modelValue", v),
});
const checked = computed({
  get: () => props.modelValue === true,
  set: (v: boolean) => emit("update:modelValue", v),
});
const selected = computed(() =>
  Array.isArray(props.modelValue) ? props.modelValue : [],
);
function toggle(value: string, on: boolean) {
  const next = selected.value.filter((v) => v !== value);
  if (on) next.push(value);
  emit("update:modelValue", next);
}
const options = computed(() => {
  const list = (props.field.picklistValues ?? []).filter((p) => p.active);
  const current = text.value;
  // An unrestricted picklist may hold a value that is not in its list.
  if (current && !list.some((p) => p.value === current))
    return [
      ...list,
      { value: current, label: current, active: true, defaultValue: false },
    ];
  return list;
});
const lookups = computed(() => {
  const current = text.value;
  if (current && !props.lookupOptions.some((o) => o.id === current))
    return [...props.lookupOptions, { id: current, name: current }];
  return props.lookupOptions;
});
const textArea = computed(() => props.field.type === "textarea");
const inputType = computed(
  () =>
    (
      ({
        email: "email",
        phone: "tel",
        date: "date",
        datetime: "datetime-local",
      }) as Record<string, string>
    )[props.field.type] ?? "text",
);
// Formatted preview of a valid number in the UI locale and tenant currency.
const preview = computed(() => {
  if (!isNumeric(props.field) || text.value === "") return "";
  const [precision, scale] = bounds(props.field);
  if (validateDecimal(text.value, precision, scale) !== "ok") return "";
  return formatValue(
    props.field,
    text.value.trim(),
    props.locale,
    props.currency,
  );
});
function checkDecimal(event: Event) {
  const input = event.target as HTMLInputElement;
  const [precision, scale] = bounds(props.field);
  const result =
    input.value === "" ? "ok" : validateDecimal(input.value, precision, scale);
  input.setCustomValidity(
    result === "syntax"
      ? t("field.invalidNumber")
      : result === "range"
        ? t("field.numberRange")
        : "",
  );
}
</script>

<template>
  <label v-if="readonly" class="read-only"
    >{{ label
    }}<output :data-field="field.name">{{
      formatValue(field, display, locale, currency)
    }}</output></label
  >
  <label v-else-if="field.type === 'boolean'" class="checkbox"
    ><input v-model="checked" type="checkbox" :disabled="disabled" />{{
      label
    }}</label
  >
  <fieldset v-else-if="field.type === 'multipicklist'" class="multi">
    <legend>{{ label }}</legend>
    <label
      v-for="option in (field.picklistValues ?? []).filter((p) => p.active)"
      :key="option.value"
      ><input
        type="checkbox"
        :checked="selected.includes(option.value)"
        :disabled="disabled"
        @change="
          toggle(option.value, ($event.target as HTMLInputElement).checked)
        "
      />{{ option.label }}</label
    >
  </fieldset>
  <label v-else-if="field.type === 'picklist'"
    >{{ label
    }}<select v-model="text" :required="!field.nillable" :disabled="disabled">
      <option v-if="field.nillable" value="">{{ t("field.none") }}</option>
      <option
        v-for="option in options"
        :key="option.value"
        :value="option.value"
      >
        {{ option.label }}
      </option>
    </select></label
  >
  <label v-else-if="field.type === 'reference' && lookupOptions.length"
    >{{ label
    }}<select v-model="text" :required="!field.nillable" :disabled="disabled">
      <option v-if="field.nillable" value="">{{ t("field.none") }}</option>
      <option v-for="option in lookups" :key="option.id" :value="option.id">
        {{
          option.name === option.id
            ? option.id
            : `${option.name} (${option.id})`
        }}
      </option>
    </select></label
  >
  <label v-else-if="isNumeric(field)"
    >{{ label
    }}<input
      v-model="text"
      type="text"
      inputmode="decimal"
      autocomplete="off"
      :required="!field.nillable"
      :disabled="disabled"
      @input="checkDecimal"
    /><small v-if="preview" class="hint" data-testid="number-preview">{{
      preview
    }}</small></label
  >
  <label v-else-if="textArea"
    >{{ label
    }}<textarea
      v-model="text"
      :required="!field.nillable"
      :maxlength="field.length || undefined"
      :disabled="disabled"
    ></textarea>
  </label>
  <label v-else
    >{{ label
    }}<input
      v-model="text"
      :type="inputType"
      :inputmode="field.type === 'url' ? 'url' : undefined"
      :required="!field.nillable"
      :maxlength="field.length || undefined"
      :placeholder="
        field.type === 'reference' ? t('field.lookupManual') : undefined
      "
      :disabled="disabled"
    /><small v-if="field.type === 'datetime'" class="hint">{{
      t("field.timeZone")
    }}</small></label
  >
</template>
