<script setup>
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { state, toast, refresh, loadSources } from '../store.js'
import { api } from '../api.js'
import TrackTable from '../components/TrackTable.vue'

const totalMin = computed(() => Math.round(state.tracks.reduce((a, t) => a + (t.duration || 0), 0) / 60))
let poll = null

async function scan() {
  const dir = state.scanDir.trim();
  if (!dir) { toast('请输入目录路径', 'err'); return; }
  state.scanMsg = '扫描中…';
  try {
    await api('/api/scan', { method: 'POST', body: JSON.stringify({ dir }) });
    toast('扫描已开始，边扫边加载', 'ok');
    startPoll();
  } catch (e) { toast('扫描失败: ' + e.message, 'err'); state.scanMsg = ''; }
}

async function tick() {
  try {
    const st = await api('/api/scan/status');
    state.scanStatus = st;
    if (st.running) {
      try { state.tracks = await api('/api/tracks'); } catch (e) {}
      state.scanMsg = '扫描中… 已加载 ' + st.added + ' 首 / 发现 ' + st.total + ' 个音频';
    } else {
      state.scanMsg = '扫描完成：共 ' + st.added + ' 首' + (st.error ? '（' + st.error + '）' : '');
      stopPoll();
      await refresh();
    }
  } catch (e) { /* 轮询容错 */ }
}

function startPoll() { stopPoll(); poll = setInterval(tick, 800); }
function stopPoll() { if (poll) { clearInterval(poll); poll = null; } }

onMounted(async () => {
  await loadSources();
  await refresh();
  if (state.scanStatus.running) startPoll();
});
onBeforeUnmount(stopPoll);
</script>

<template>
  <div>
    <div class="cards">
      <div class="card"><div class="num">{{ state.tracks.length }}</div><div class="lab">曲目</div></div>
      <div class="card"><div class="num">{{ state.dupGroups.length }}</div><div class="lab">重复分组</div></div>
      <div class="card"><div class="num">{{ totalMin }}</div><div class="lab">总时长(分钟)</div></div>
    </div>

    <div class="panel">
      <h2>扫描目录（异步加载，边扫边出）</h2>
      <div class="row">
        <input type="text" v-model="state.scanDir" placeholder="容器内音频目录，如 /music" @keydown.enter="scan" />
        <button @click="scan" :disabled="state.scanStatus.running">{{ state.scanStatus.running ? '扫描中…' : '扫描' }}</button>
      </div>
      <div class="loading">{{ state.scanMsg }}</div>
    </div>

    <div class="panel">
      <h2>曲目列表</h2>
      <div style="max-height:520px;overflow:auto"><TrackTable /></div>
    </div>
  </div>
</template>
