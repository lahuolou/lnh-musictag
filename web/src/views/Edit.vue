<script setup>
import { onMounted, watch } from 'vue'
import { state, refresh, loadSources } from '../store.js'
import TrackTable from '../components/TrackTable.vue'
import EditPanel from '../components/EditPanel.vue'

function routeId() {
  const parts = location.hash.split('/');
  return parts[2] || '';
}
function applyId() {
  const id = routeId();
  if (id) state.currentId = decodeURIComponent(id);
  else if (state.tracks.length && !state.currentId) state.currentId = state.tracks[0].id;
}

onMounted(async () => {
  await loadSources();
  await refresh();
  applyId();
});
watch(() => location.hash, applyId);
</script>

<template>
  <div class="grid2" style="align-items:start">
    <div class="panel">
      <h2>曲目</h2>
      <div style="max-height:620px;overflow:auto"><TrackTable /></div>
    </div>
    <div class="panel"><h2>编辑 / 刮削</h2><EditPanel /></div>
  </div>
</template>
