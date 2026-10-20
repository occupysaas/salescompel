<script setup lang="ts">
import { t } from "../i18n";
import { evidence } from "./evidence";
</script>
<template>
  <div class="p-title">
    <div>
      <p class="p-eyebrow">{{ t("preview.designNotes") }}</p>
      <h1>{{ t("preview.evidence") }}</h1>
      <p>{{ t("preview.evidenceIntro") }}</p>
    </div>
  </div>
  <section v-for="entry in evidence" :key="entry.id" class="p-panel p-evidence">
    <p class="p-eyebrow">{{ t(entry.screen) }}</p>
    <h2>{{ t(entry.action) }}</h2>
    <div class="p-endpoints">
      <div v-for="api in entry.api" :key="api.method + api.path">
        <span>{{ api.method }}</span> <code>{{ api.path }}</code>
      </div>
      <p v-if="!entry.api.length">{{ t("preview.noApi") }}</p>
    </div>
    <dl>
      <dt>{{ t("preview.reference") }}</dt>
      <dd>
        <template v-for="ref in entry.reference" :key="ref.source"
          ><p>
            {{ t(ref.status) }}<br /><code>{{ ref.source }}</code>
          </p></template
        ><span v-if="!entry.reference.length">{{
          t("preview.noObservation")
        }}</span>
      </dd>
      <dt>{{ t("preview.candidate") }}</dt>
      <dd>
        {{ t(entry.candidate.status) }}<br /><code>{{
          entry.candidate.source
        }}</code>
      </dd>
      <dt>{{ t("preview.simulation") }}</dt>
      <dd>{{ t(entry.simulation) }}</dd>
      <dt>{{ t("preview.limits") }}</dt>
      <dd>{{ t(entry.limitations) }}</dd>
    </dl>
  </section>
</template>
