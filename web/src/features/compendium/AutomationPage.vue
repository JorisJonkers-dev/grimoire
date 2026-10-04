<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { getAutomationCoverageOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { kindLabel } from './kinds'

const coverage = useQuery(getAutomationCoverageOptions())
</script>

<template>
  <main class="g-page automation">
    <header class="g-headline">
      <span class="g-eyebrow">About</span>
      <h1>Automation coverage</h1>
    </header>
    <p>
      How much of each kind of entry the rules compute. Anything not yet automated is resolved by the DM from a prompt at the
      table.
    </p>
    <p v-if="coverage.isPending.value">Counting…</p>
    <p v-else-if="coverage.isError.value" role="alert">The report could not be loaded. Try again shortly.</p>
    <div v-else class="scroll" role="group" aria-label="Table of automation coverage" tabindex="0">
      <table data-testid="automation-table">
        <thead>
          <tr>
            <th scope="col">Kind</th>
            <th scope="col">Entries</th>
            <th scope="col">Full</th>
            <th scope="col">Partial</th>
            <th scope="col">Manual</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in coverage.data.value" :key="row.kind">
            <th scope="row">{{ row.kind === 'spell' ? 'Spells' : kindLabel(row.kind) }}</th>
            <td>{{ row.total }}</td>
            <td>{{ row.full }}</td>
            <td>{{ row.partial }}</td>
            <td>{{ row.manual }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </main>
</template>

<style scoped>
.automation {
  width: 100%;
  padding: 24px var(--gutter);
  box-sizing: border-box;
}
/* A table wider than a phone scrolls on its own, not the page. */
.scroll {
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  padding: 8px 6px;
  border-bottom: 1px solid var(--color-line);
  text-align: right;
}
th[scope='row'],
th:first-child {
  text-align: left;
}
</style>
