<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { state, toast, refresh, loadSources } from '../store.js'
import { api } from '../api.js'

const q = ref('')
const filter = ref('all')
const scanDir = ref(state.scanDir)
const scanMsg = ref('')
let poll = null

// 繁体专有字（简体中不存在），用于“繁体”筛选
const TRAD_SET = '這那們時間說學會後點對與應來為愛樂讓體識認請語質護車風華東開關馬鳥魚龍長發興義氣無見覺親觀邊還進過這這麼樣種頭裏條單雙萬億賣買價錢銀銅鐵鋼鹽糖湯飯餐館樓層廳室庫廚門窗牆橋輪機械電腦網線紙筆書畫際標準規則條約證據賬號碼鍵盤螢幕顯示頻錄影攝像鏡頭電池插座按鈕遙控';

function isGarbled(t) {
  const s = [t.fileName, t.tags.TITLE || '', t.tags.ARTIST || '', t.tags.ALBUM || ''].join(' ');
  return /[\uFFFD]/.test(s) || /[ÃÂ][\u0080-\u00BF]/.test(s) || /â€[™“”Œ]/.test(s) || /[åæçèéêëìíîï][\u0080-\u00BF]{2}/.test(s);
}
function isTrad(t) {
  const s = [t.tags.TITLE || '', t.tags.ARTIST || '', t.tags.ALBUM || ''].join(' ');
  if (!/[\u4E00-\u9FFF]/.test(s)) return false;
  return [...TRAD_SET].some(c => s.includes(c));
}

// 表格列表：全部曲目（可按关键词过滤 + 按问题类型筛选）
const filtered = computed(() => {
  const kw = q.value.trim().toLowerCase();
  let list = state.tracks;
  if (kw) {
    list = list.filter(t =>
      [t.tags.TITLE || '', t.tags.ARTIST || '', t.tags.ALBUM || '', t.fileName].join(' ').toLowerCase().includes(kw));
  }
  if (filter.value === 'garbled') list = list.filter(isGarbled);
  else if (filter.value === 'trad') list = list.filter(isTrad);
  else if (filter.value === 'nolyric') list = list.filter(t => !hasLyrics(t));
  else if (filter.value === 'nocover') list = list.filter(t => !t.hasCover);
  else if (filter.value === 'noartist') list = list.filter(t => !(t.tags.ARTIST || '').trim());
  return list;
});
const filterOpts = [
  { id: 'all', label: '全部' },
  { id: 'garbled', label: '乱码' },
  { id: 'trad', label: '繁体' },
  { id: 'nolyric', label: '无歌词' },
  { id: 'nocover', label: '无封面' },
  { id: 'noartist', label: '无艺术家' }
];
const filtering = computed(() => q.value.trim() !== '' || filter.value !== 'all');

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
      scanMsg.value = '扫描中… 已加载 ' + st.added + ' 首 / 发现 ' + st.total + (st.skipped ? '（智能跳过 ' + st.skipped + '）' : '');
    } else {
      scanMsg.value = '扫描完成：共 ' + st.added + ' 首' + (st.skipped ? '（智能跳过 ' + st.skipped + '）' : '') + (st.error ? '（' + st.error + '）' : '');
      stopPoll();
      await refresh();
    }
  } catch (e) { /* 轮询容错 */ }
}
function startPoll() { stopPoll(); poll = setInterval(tick, 800); }
function stopPoll() { if (poll) { clearInterval(poll); poll = null; } }

async function togglePause() {
  try {
    const r = await api('/api/scan/pause', { method: 'POST', body: JSON.stringify({ paused: !state.scanStatus.paused }) });
    state.scanStatus = r;
    toast(r.paused ? '已暂停扫描' : '已继续扫描', 'ok');
  } catch (e) { toast('操作失败: ' + e.message, 'err'); }
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
        <button v-if="state.scanStatus.running" class="ghost sm" @click="togglePause">{{ state.scanStatus.paused ? '继续' : '暂停' }}</button>
      </div>
      <div class="row" style="margin-top:8px">
        <input type="text" v-model="q" placeholder="输入即搜索 标题/艺术家/专辑/文件名" style="min-width:0" />
      </div>
      <div class="filters" style="margin-top:8px">
        <button v-for="f in filterOpts" :key="f.id" class="chip" :class="{ on: filter === f.id }" @click="filter = f.id">{{ f.label }}</button>
      </div>
      <div class="loading" style="margin-top:6px">{{ scanMsg }}{{ state.scanStatus.paused ? '（已暂停）' : '' }}</div>
    </div>

    <div class="ml-head">
      {{ filtering ? '匹配 ' + filtered.length + ' / 共 ' + state.tracks.length : '曲目 ' + state.tracks.length }} · 勾选后到「批量」补全 · 点击行进「编辑」
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
