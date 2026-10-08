import { reactive } from 'vue'
import { api } from './api.js'

export const state = reactive({
  authed: false,
  user: '',
  route: location.hash.slice(1) || '/',
  activeTab: 'list',
  tracks: [],
  selected: new Set(),
  sources: [],
  mbRes: [],
  currentId: null,
  dupGroups: [],
  scanDir: '/music',
  scanMsg: '',
  scanStatus: { running: false, paused: false, added: 0, total: 0, done: 0, skipped: 0, error: '' },
  bSource: 'auto',
  bFetchCover: true,
  bFetchLyrics: false,
  batchBusy: false,
  batchJob: null,
  toast: { msg: '', type: '', show: false, _tm: null }
})

export function go(p) { location.hash = p }

export function cur() { return state.tracks.find(t => t.id === state.currentId) || null }

export function toast(msg, type = '') {
  state.toast.msg = msg;
  state.toast.type = type;
  state.toast.show = true;
  clearTimeout(state.toast._tm);
  state.toast._tm = setTimeout(() => (state.toast.show = false), 2600);
}

// 把批量完成的最新曲目增量合并进列表：按 id 就地替换，
// 不重拉全量列表，因此不打断滚动/勾选/编辑中的状态。
export function mergeTracks(list = []) {
  if (!list.length) return;
  const byId = new Map(list.map(t => [t.id, t]));
  state.tracks = state.tracks.map(t => byId.get(t.id) || t);
  // 合并可能带来从未见过的曲目（如新增）
  const known = new Set(state.tracks.map(t => t.id));
  for (const t of list) if (!known.has(t.id)) state.tracks.push(t);
}

let _batchTimer = null;

// 全局批量轮询：批量任务在后台运行，切到任意页面进度都保持可见。
// 每次轮询把后端返回的增量 updated 合并进列表，列表实时更新、不整表刷新。
export async function startBatchPoll(jobId) {
  stopBatchPoll();
  state.batchBusy = true;
  const poll = async () => {
    try {
      const job = await api('/api/scrape/jobs/' + encodeURIComponent(jobId));
      state.batchJob = job;
      mergeTracks(job.updated || []);
      if (job.status === 'done') {
        stopBatchPoll();
        const ok = (job.results || []).filter(r => r.ok).length;
        const skipped = (job.results || []).filter(r => r.ok && /智能跳过/.test(r.message || '')).length;
        toast('批量刮削完成：' + ok + '/' + job.total + ' 成功' + (skipped ? '（智能跳过 ' + skipped + '）' : ''), ok === job.total ? 'ok' : 'err');
        state.selected.clear();
        state.batchJob = null;
        await refresh(); // 兜底同步一次，保证与后端完全一致
      }
    } catch (e) {
      stopBatchPoll();
      state.batchJob = null;
      state.batchBusy = false;
      toast('批量进度查询失败: ' + e.message, 'err');
    }
  };
  await poll();
  if (state.batchJob) _batchTimer = setInterval(poll, 1000);
}

export function stopBatchPoll() {
  if (_batchTimer) { clearInterval(_batchTimer); _batchTimer = null; }
  state.batchBusy = false;
}

export async function loadSources() {
  try { state.sources = await api('/api/scrape/sources'); }
  catch (e) {
    state.sources = [
      { name: 'auto', label: '自动' }, { name: 'musicbrainz', label: 'MusicBrainz' },
      { name: 'itunes', label: 'iTunes' }, { name: 'netease', label: '网易云' }, { name: 'qq', label: 'QQ音乐' }
    ];
  }
}

export async function refresh() {
  // 静默刷新：更新列表不弹窗、不打断操作
  try { state.tracks = await api('/api/tracks'); } catch (e) { /* 静默 */ }
  await loadDups();
}

export async function loadDups() {
  try {
    state.dupGroups = (await api('/api/duplicates')) || [];
  } catch (e) { /* dedup is best-effort */ }
}

export async function logout() {
  try { await api('/api/logout', { method: 'POST' }); } catch (e) { /* ignore */ }
  state.authed = false;
  state.route = '/';
  location.hash = '/';
}
