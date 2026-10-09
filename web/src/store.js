import { reactive } from 'vue'
import { api } from './api.js'
import { t } from './i18n.js'

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
  toolJob: null, // 通用工具任务（音频识别/格式转换/乱码修复/简繁转换）进度
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
        toast(t('batchFinish', { ok, t: job.total, s: skipped ? t('skipped', { n: skipped }) : '' }), ok === job.total ? 'ok' : 'err');
        state.selected.clear();
        state.batchJob = null;
        await refresh(); // 兜底同步一次，保证与后端完全一致
      }
    } catch (e) {
      stopBatchPoll();
      state.batchJob = null;
      state.batchBusy = false;
      toast(t('batchPollFail', { m: e.message }), 'err');
    }
  };
  await poll();
  if (state.batchJob) _batchTimer = setInterval(poll, 1000);
}

export function stopBatchPoll() {
  if (_batchTimer) { clearInterval(_batchTimer); _batchTimer = null; }
  state.batchBusy = false;
}

let _toolTimer = null;

// 通用工具任务轮询：音频识别 / 格式转换 / 乱码修复 / 简繁转换。
// 与批量补全共用“后台运行、进度全局可见”的模式，任务完成后置 finished，
// 由发起组件负责 toast、清勾选与刷新。
export async function startToolJob(jobId, total) {
  stopToolJob();
  state.toolJob = { id: jobId, total, done: 0, current: '', status: 'running', ok: 0, errors: [], finished: false };
  const poll = async () => {
    try {
      const j = await api('/api/jobs/' + encodeURIComponent(jobId));
      const tj = state.toolJob;
      if (!tj) return;
      tj.done = j.done;
      tj.current = j.current || '';
      if (j.status === 'done') {
        stopToolJob();
        tj.status = 'done';
        tj.finished = true;
        tj.ok = (j.results || []).filter(r => r.ok).length;
        tj.errors = (j.results || []).filter(r => !r.ok && r.message);
      }
    } catch (e) { /* 瞬时网络抖动忽略，下一轮再试 */ }
  };
  await poll();
  if (state.toolJob && !state.toolJob.finished) _toolTimer = setInterval(poll, 800);
}

export function stopToolJob() {
  if (_toolTimer) { clearInterval(_toolTimer); _toolTimer = null; }
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
