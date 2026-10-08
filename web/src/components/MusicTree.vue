<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { state, toast, refresh, loadSources } from '../store.js'
import { api } from '../api.js'

const q = ref('')
const filter = ref('all')
const scanDir = ref(state.scanDir)
const scanMsg = ref('')
let poll = null

// 排序状态（默认不排序=保持扫描顺序；扫描进行中强制保持扫描顺序，暂停/完成后可点表头排序）
const sortKey = ref(null) // null = 扫描顺序
const sortDir = ref(1) // 1=升序 -1=降序
const pageSize = 200
const visible = ref(pageSize)

// 繁体专有字（简体中不存在），用于“繁体”筛选
const TRAD_SET = '這那們時間說學會後點對與應來為愛樂讓體識認請語質護車風華東開關馬鳥魚龍長發興義氣無見覺親觀邊還進過這這麼樣種頭裏條單雙萬億賣買價錢銀銅鐵鋼鹽糖湯飯餐館樓層廳室庫廚門窗牆橋輪機械電腦網線紙筆書畫際標準規則條約證據賬號碼鍵盤螢幕顯示頻錄影攝像鏡頭電池插座按鈕遙控';

// 归一化：全角→半角、全角空格→普通空格、小写、去首尾空白（兼容中文输入习惯）
function norm(s) {
  return (s || '').replace(/[\uFF01-\uFF5E]/g, ch => String.fromCharCode(ch.charCodeAt(0) - 0xFEE0))
    .replace(/\u3000/g, ' ').trim().toLowerCase();
}

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
  const kw = norm(q.value);
  let list = state.tracks;
  if (kw) {
    list = list.filter(t =>
      norm([t.tags.TITLE || '', t.tags.ARTIST || '', t.tags.ALBUM || '', t.fileName].join(' ')).includes(kw));
  }
  if (filter.value === 'garbled') list = list.filter(isGarbled);
  else if (filter.value === 'trad') list = list.filter(isTrad);
  else if (filter.value === 'nolyric') list = list.filter(t => !hasLyrics(t) && !t.hasLrcFile);
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

// 文件名显示名：去尾部音频后缀（发如雪.mp3.flac -> 发如雪）
function baseName(t) {
  let n = t.fileName || '';
  while (true) {
    const m = n.match(/\.(mp3|flac|wav|m4a|aac|ogg|opus|ape|wma|aiff|mka)$/i);
    if (!m) break;
    n = n.slice(0, n.length - m[0].length);
  }
  return n;
}
// 从“艺术家 - 标题”文件名解析艺术家（标签为空时回退显示）
function nameArtist(t) {
  const m = (t.fileName || '').replace(/\.(mp3|flac|wav|m4a|aac|ogg|opus|ape|wma|aiff|mka)$/i, '').match(/^(.*?)[\s\-—–]+([^\-—–]+)$/);
  return m ? m[1].trim() : '';
}
function displayTitle(t) { return (t.tags.TITLE || '').trim() || baseName(t); }
function displayArtist(t) { return (t.tags.ARTIST || '').trim() || nameArtist(t); }

// 排序
function sortVal(t, k) {
  switch (k) {
    case 'title': return displayTitle(t);
    case 'artist': return displayArtist(t);
    case 'album': return t.tags.ALBUM || '';
    case 'albumartist': return t.tags.ALBUMARTIST || '';
    case 'year': return parseInt(t.tags.DATE || '', 10) || 0;
    case 'genre': return t.tags.GENRE || '';
    case 'size': return t.size || 0;
    case 'duration': return t.duration || 0;
  }
  return '';
}
const sorted = computed(() => {
  const list = filtered.value.slice();
  // 扫描进行中（未暂停）：强制保持扫描顺序（先扫到的在前，递增在后）
  if (state.scanStatus.running && !state.scanStatus.paused) return list;
  const k = sortKey.value;
  if (!k) return list; // 未选择排序键：保持扫描顺序
  const d = sortDir.value;
  list.sort((a, b) => {
    const va = sortVal(a, k), vb = sortVal(b, k);
    let c;
    if (typeof va === 'number' && typeof vb === 'number') c = va - vb;
    else c = String(va).localeCompare(String(vb), 'zh-Hans-CN', { numeric: true, sensitivity: 'base' });
    return c * d;
  });
  return list;
});
function setSort(k) {
  if (sortKey.value === k) sortDir.value *= -1;
  else { sortKey.value = k; sortDir.value = 1; }
}
function sortIcon(k) { return sortKey.value === k ? (sortDir.value === 1 ? '▲' : '▼') : ''; }

// 分页渲染：大曲库不卡（滚动到底自动加载更多）
const shown = computed(() => sorted.value.slice(0, visible.value));
watch([q, filter, sortKey, sortDir], () => { visible.value = pageSize; });
function onScroll(e) {
  const el = e.target;
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 200 && visible.value < sorted.value.length) {
    visible.value += pageSize;
  }
}

function coverUrl(id) { return '/api/tracks/' + encodeURIComponent(id) + '/cover'; }
function hasLyrics(t) { return !!(t.tags.LYRICS || '').trim(); }
function hasLrc(t) { return /\[\d{1,2}:\d{2}/.test(t.tags.LYRICS || ''); }
function lrcMark(t) {
  const a = hasLrc(t), b = !!t.hasLrcFile;
  return a && b ? '双' : a ? '有' : b ? '外' : '无';
}
function fmtSize(s) {
  if (!s) return '';
  const mb = s / 1048576;
  return mb >= 1024 ? (mb / 1024).toFixed(1) + ' GB' : mb.toFixed(1) + ' MB';
}

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
        <input type="text" v-model="q" placeholder="输入即搜索 标题/艺术家/专辑/文件名（自动忽略全角/大小写）" style="min-width:0" />
      </div>
      <div class="filters" style="margin-top:8px">
        <button v-for="f in filterOpts" :key="f.id" class="chip" :class="{ on: filter === f.id }" @click="filter = f.id">{{ f.label }}</button>
      </div>
      <div class="loading" style="margin-top:6px">{{ scanMsg }}{{ state.scanStatus.paused ? '（已暂停）' : '' }}</div>
    </div>

    <div class="ml-head">
      {{ filtering ? '匹配 ' + filtered.length + ' / 共 ' + state.tracks.length : '曲目 ' + state.tracks.length }} · 扫描中按扫描顺序排列，暂停/完成后可点标题、艺术家等表头按 A-Z 0-9 排序 · 勾选后到「批量」补全 · 点击行进「编辑」
    </div>

    <div class="tree-scroll" @scroll.passive="onScroll">
      <table class="mtbl">
        <thead>
          <tr>
            <th style="width:30px"><input type="checkbox" class="tchk" :checked="isAllSel()" @change="toggleAll" /></th>
            <th style="width:46px">封面</th>
            <th class="th-sort" @click="setSort('title')">标题 {{ sortIcon('title') }}</th>
            <th class="th-sort" @click="setSort('artist')">艺术家 {{ sortIcon('artist') }}</th>
            <th class="th-sort" @click="setSort('album')">专辑 {{ sortIcon('album') }}</th>
            <th class="th-sort" @click="setSort('albumartist')">专辑艺术家 {{ sortIcon('albumartist') }}</th>
            <th style="width:52px">歌词</th>
            <th style="width:46px">LRC</th>
            <th class="th-sort" style="width:60px" @click="setSort('year')">年份 {{ sortIcon('year') }}</th>
            <th class="th-sort" @click="setSort('genre')">风格 {{ sortIcon('genre') }}</th>
            <th class="th-sort" style="width:64px" @click="setSort('size')">大小 {{ sortIcon('size') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in shown" :key="t.id" :class="{ on: state.selected.has(t.id) }" @click="openTrack(t)">
            <td><input type="checkbox" class="tchk" :checked="state.selected.has(t.id)" @click.stop @change="toggleSel(t, $event)" /></td>
            <td @click.stop>
              <img v-if="t.hasCover" class="cov" :src="coverUrl(t.id)" loading="lazy" @click.stop="openTrack(t)" />
              <span v-else class="covph">♪</span>
            </td>
            <td :title="t.tags.TITLE ? t.tags.TITLE : baseName(t)">{{ displayTitle(t) }}</td>
            <td :title="t.tags.ARTIST ? t.tags.ARTIST : nameArtist(t)">{{ displayArtist(t) }}</td>
            <td>{{ t.tags.ALBUM || '' }}</td>
            <td>{{ t.tags.ALBUMARTIST || '' }}</td>
            <td>{{ hasLyrics(t) ? '有' : '无' }}</td>
            <td :title="t.hasLrcFile ? '同目录外挂 .lrc 歌词' : ''">{{ lrcMark(t) }}</td>
            <td>{{ t.tags.DATE || '' }}</td>
            <td>{{ t.tags.GENRE || '' }}</td>
            <td class="muted">{{ fmtSize(t.size) }}</td>
          </tr>
          <tr v-if="!shown.length">
            <td colspan="11" class="muted" style="padding:16px;text-align:center">
              {{ state.tracks.length ? '没有匹配的曲目，换个关键词或筛选试试。' : '还没有曲目，先在上方输入目录扫描。' }}
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="visible < sorted.length" class="muted" style="padding:8px;text-align:center;font-size:12px">
        已显示 {{ shown.length }} / {{ sorted.length }} 首，继续滚动加载更多…
      </div>
    </div>
  </div>
</template>
