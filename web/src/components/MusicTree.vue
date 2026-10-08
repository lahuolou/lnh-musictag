<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { state, toast, refresh, loadSources } from '../store.js'
import { api } from '../api.js'

const q = ref('')
const scanDir = ref(state.scanDir)
const scanMsg = ref('')
let poll = null

// 表格列表：全部曲目（可按关键词过滤）
const filtered = computed(() => {
  const kw = q.value.trim().toLowerCase();
  if (!kw) return state.tracks;
  return state.tracks.filter(t =>
    [t.tags.TITLE || '', t.tags.ARTIST || '', t.tags.ALBUM || '', t.fileName].join(' ').toLowerCase().includes(kw));
});

// 文件名（去音频扩展名），如 陈小春 - 街角的晚风.flac → 陈小春 - 街角的晚风
function trackName(t) {
  return (t.fileName || '').replace(/\.(mp3|flac|m4a|aac|ogg|opus|wav|wma|ape|mpc|aiff|mka)$/i, '');
}
function coverUrl(id) { return '/api/tracks/' + encodeURIComponent(id) + '/cover'; }
function hasLyrics(t) { return !!(t.tags.LYRICS || '').trim(); }
function hasLrc(t) { return /\[\d{1,2}:\d{2}/.test(t.tags.LYRICS || ''); }
function isAllSel() { return filtered.value.length > 0 && filtered.value.every(t => state.selected.has(t.id)); }
function toggleAll(e) {
  if (e.target.checked) filtered.value.forEach(t => state.selected.add(t.id));
  else filtered.value.forEach(t => state.selected.delete(t.id));
}
function toggleSel(t, ev) {
  if (ev.target.checked) state.selected.add(t.id); else state.selected.delete(t.id);
}
function openTrack(t) {
  state.currentId = t.id;
  state.activeTab = 'edit';
}

async function scan() {
  const dir = scanDir.value.trim();
  if (!dir) { toast('请输入目录路径', 'err'); return; }
  scanMsg.value = '扫描中…';
  try {
    await api('/api/scan', { method: 'POST', body: JSON.stringify({ dir }) });
    toast('扫描已开始，边扫边出', 'ok');
    startPoll();
  } catch (e) { toast('扫描失败: ' + e.message, 'err'); scanMsg.value = ''; }
}
async function tick() {
  try {
    const st = await api('/api/scan/status');
    state.scanStatus = st;
    if (st.running) {
      try { state.tracks = await api('/api/tracks'); } catch (e) {}
      scanMsg.value = '扫描中… 已加载 ' + st.added + ' 首 / 发现 ' + st.total;
    } else {
      scanMsg.value = '扫描完成：共 ' + st.added + ' 首' + (st.error ? '（' + st.error + '）' : '');
      stopPoll();
      await refresh();
    }
  } catch (e) { /* 轮询容错 */ }
}
function startPoll() { stopPoll(); poll = setInterval(tick, 800); }
function stopPoll() { if (poll) { clearInterval(poll); poll = null; } }
function doSearch() {
  if (!q.value.trim()) { toast('请输入搜索关键词', 'err'); return; }
  toast('搜索完成：共 ' + filtered.value.length + ' 首匹配', 'ok');
}

onMounted(async () => { await loadSources(); await refresh(); if (state.scanStatus.running) startPoll(); });
onBeforeUnmount(stopPoll);
</script>

<template>
  <div class="music-list">
    <div class="ml-toolbar">
      <div class="row">
        <input type="text" v-model="scanDir" placeholder="容器内音频目录，如 /music" @keydown.enter="scan" style="min-width:0" />
        <button class="sm" @click="scan" :disabled="state.scanStatus.running">{{ state.scanStatus.running ? '扫描中' : '扫描' }}</button>
      </div>
      <div class="row" style="margin-top:8px">
        <input type="text" v-model="q" placeholder="搜索标题/艺术家/专辑/文件名" style="min-width:0" @keydown.enter="doSearch" />
        <button class="ghost sm" @click="doSearch">搜索</button>
      </div>
      <div class="loading">{{ scanMsg }}</div>
    </div>

    <div class="ml-head">
      曲目 {{ filtered.length }} · 勾选后到「批量」补全 · 点击行进「编辑」
    </div>

    <div class="tree-scroll">
      <table class="mtbl">
        <thead>
          <tr>
            <th style="width:30px"><input type="checkbox" class="tchk" :checked="isAllSel()" @change="toggleAll" /></th>
            <th style="width:46px">封面</th>
            <th>文件名</th>
            <th>标题</th>
            <th>艺术家</th>
            <th>专辑</th>
            <th>专辑艺术家</th>
            <th style="width:52px">歌词</th>
            <th style="width:46px">LRC</th>
            <th style="width:56px">年份</th>
            <th>风格</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in filtered" :key="t.id" :class="{ on: state.selected.has(t.id) }">
            <td><input type="checkbox" class="tchk" :checked="state.selected.has(t.id)" @click.stop @change="toggleSel(t, $event)" /></td>
            <td>
              <img v-if="t.hasCover" class="cov" :src="coverUrl(t.id)" @click.stop="openTrack(t)" />
              <span v-else class="covph">♪</span>
            </td>
            <td class="fn" @click="openTrack(t)" :title="t.path">{{ trackName(t) }}</td>
            <td>{{ t.tags.TITLE || '' }}</td>
            <td>{{ t.tags.ARTIST || '' }}</td>
            <td>{{ t.tags.ALBUM || '' }}</td>
            <td>{{ t.tags.ALBUMARTIST || '' }}</td>
            <td>{{ hasLyrics(t) ? '有' : '无' }}</td>
            <td>{{ hasLrc(t) ? '有' : '无' }}</td>
            <td>{{ t.tags.DATE || '' }}</td>
            <td>{{ t.tags.GENRE || '' }}</td>
          </tr>
          <tr v-if="!filtered.length"><td colspan="11" class="muted" style="padding:16px;text-align:center">还没有曲目，先在上方输入目录扫描。</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
