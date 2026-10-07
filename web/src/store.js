import { reactive } from 'vue'
import { api } from './api.js'

export const state = reactive({
  authed: false,
  user: '',
  route: location.hash.slice(1) || '/',
  tracks: [],
  selected: new Set(),
  sources: [],
  mbRes: [],
  currentId: null,
  dupGroups: [],
  fpActive: false,
  scanDir: '/music',
  scanMsg: '',
  scanStatus: { running: false, added: 0, total: 0, done: 0, error: '' },
  bSource: 'auto',
  bFetchCover: true,
  bFetchLyrics: false,
  batchBusy: false,
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
  try { state.tracks = await api('/api/tracks'); }
  catch (e) { toast('加载失败: ' + e.message, 'err'); return; }
  await loadDups();
}

export async function loadDups() {
  try {
    const groups = (await api('/api/duplicates')) || [];
    state.dupGroups = groups;
    state.fpActive = groups.some(g => g.method === 'fingerprint');
  } catch (e) { /* dedup is best-effort */ }
}

export async function logout() {
  try { await api('/api/logout', { method: 'POST' }); } catch (e) { /* ignore */ }
  state.authed = false;
  state.route = '/';
  location.hash = '/';
}
