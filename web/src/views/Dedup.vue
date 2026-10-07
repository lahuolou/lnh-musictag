<script setup>
import { onMounted } from 'vue'
import { refresh } from '../store.js'
import DedupPanel from '../components/DedupPanel.vue'

onMounted(async () => { await refresh(); });
</script>

<template>
  <DedupPanel />
</template>
