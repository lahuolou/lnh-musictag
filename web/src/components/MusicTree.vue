<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { state, toast, refresh, loadSources } from '../store.js'
import { api } from '../api.js'
import TreeNode from './TreeNode.vue'

const q = ref('')
const scanDir = ref(state.scanDir)
const scanMsg = ref('')
const collapsed = ref(new Set())
let poll = null

// 树形结构：目录 → 曲目名（按曲目所在目录分组，两级）
const tree = computed(() => {
  const kw = q.value.trim().toLowerCase();
  let list = state.tracks;
  if (kw) {
    list = state.tracks.filter(t =>
      [t.tags.TITLE || '', t.tags.ARTIST || '', t.tags.ALBUM || '', t.fileName].join(' ').toLowerCase().includes(kw));
  }
  const dirs = new Map();
  for (const t of list) {
    const parts = t.path.split(/[\\/]+/).filter(Boolean);
    const dirPath = parts.slice(0, -1).join('/');
    let d = dirs.get(dirPath);
    if (!d) { d = { name: dirPath || '/', dir: true, path: dirPath, children: [] }; dirs.set(dirPath, d); }
    d.children.push({ name: t.fileName, dir: false, track: t });
  }
  const arr = [...dirs.values()].sort((a, b) => a.name.localeCompare(b.name, 'zh'));
  for (const d of arr) d.children.sort((a, b) => a.name.localeCompare(b.name, 'zh'));
  return arr;
})

function isCollapsed(node) { return collapsed.value.has(node.path); }
function toggle(node) {
  const s = new Set(collapsed.value);
  if (s.has(node.path)) s.delete(node.path); else s.add(node.path);
  collapsed.value = s;
}
function openTrack(t) {
  state.currentId = t.id;
  state.activeTab = 'edit';
}
function toggleSel(t, ev) {
  if (ev.target.checked) state.selected.add(t.id); else state.selected.delete(t.id);
}
function artists(t) { return (t.tags.ARTIST || '').replace(/ \/ /g, ', '); }
function folderCount(node) {
  let n = 0;
  const walk = (nd) => { if (!nd.dir) n++; else nd.children.forEach(walk); };
  walk(node);
  return n;
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

async function renameFiles() {
  const ids = state.selected.size ? [...state.selected] : [];
  if (!state.tracks.length) { toast('没有曲目', 'err'); return; }
  try {
    const res = await api('/api/rename-files', { method: 'POST', body: JSON.stringify({ ids }) });
    const n = res.results ? res.results.filter(r => r.renamed).length : 0;
    toast('重命名完成：' + n + ' 个文件已改名', 'ok');
    await refresh();
  } catch (e) { toast('重命名失败: ' + e.message, 'err'); }
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
        <input type="text" v-model="q" placeholder="搜索标题/艺术家/专辑/文件名" style="min-width:0" />
        <button class="ghost sm" @click="renameFiles" :disabled="state.scanStatus.running" title="去重复/错误音频后缀">重命名</button>
      </div>
      <div class="loading">{{ scanMsg }}</div>
    </div>

    <div class="ml-head">
      曲目 {{ state.tracks.length }} · 勾选后到「批量」补全 · 点击曲目进「编辑」
    </div>
    <div class="tree">
      <TreeNode v-for="n in tree" :key="n.path" :node="n" :depth="0"
        :collapsed="collapsed" :selected="state.selected"
        :toggle="toggle" :openTrack="openTrack" :toggleSel="toggleSel"
        :artists="artists" :folderCount="folderCount" />
    </div>
  </div>
</template>
