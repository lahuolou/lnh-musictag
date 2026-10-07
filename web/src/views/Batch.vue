<script setup>
import { onMounted } from 'vue'
import { refresh, loadSources } from '../store.js'
import BatchPanel from '../components/BatchPanel.vue'

onMounted(async () => {
  await loadSources();
  await refresh();
});
</script>

<template>
  <BatchPanel />
</template>
