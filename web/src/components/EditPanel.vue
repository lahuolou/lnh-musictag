<script setup>
import { reactive, ref, watch } from 'vue'
import { state, cur, toast, refresh } from '../store.js'
import { api } from '../api.js'

const f = ref({ title: '', artist: '', album: '', albumartist: '', genre: '', date: '', track: '', lyrics: '' })
const sQuery = ref('')
const sSource = ref('auto')
const searching = ref(false)
let lastId = null

// 手动刮削要写入的字段（可选）
const sf = reactive({
  title: true, artist: true, albumArtist: true, genre: true, year: true, trackNumber: true, lyrics: true
})
const sfCover = ref(true)

function selFields(withCover) {
  const names = [];
  const m = { title: 'title', artist: 'artist', albumArtist: 'albumArtist', genre: 'genre', year: 'year', trackNumber: 'trackNumber', lyrics: 'lyrics' };
  for (const k in m) { if (sf[k]) names.push(m[k]); }
  if (withCover) names.push('cover');
  return names;
}

function loadTrack(t) {
  if (!t) return;
  lastId = t.id;
  // 标题/艺术家为空时默认从源文件名载入（如“陈小春 - 街角的晚风.flac” → 艺术家=陈小春，标题=街角的晚风）
  let title = t.tags.TITLE || '';
  let artist = t.tags.ARTIST || '';
  if (!title) {
    const base = (t.fileName || '').replace(/\.(mp3|flac|m4a|aac|ogg|opus|wav|wma|ape|mpc|aiff|mka)$/i, '');
    const m = base.match(/^(.*?)\s*-\s*(.+)$/);
    title = m ? m[2].trim() : base.trim();
    if (!artist && m) artist = m[1].trim();
  }
  f.value = {
    title, artist,
    album: t.tags.ALBUM || '', albumartist: t.tags.ALBUMARTIST || '',
    genre: t.tags.GENRE || '', date: t.tags.DATE || '',
    track: t.tags.TRACKNUMBER || '', lyrics: t.tags.LYRICS || ''
  };
  sQuery.value = title + ' ' + artist;
  sSource.value = 'auto';
}

watch(() => state.currentId, (id) => {
  if (id === lastId) return;
  loadTrack(state.tracks.find(t => t.id === id));
});

async function save() {
  if (!cur()) return;
  const tags = {
    TITLE: f.value.title, ARTIST: f.value.artist, ALBUM: f.value.album,
    ALBUMARTIST: f.value.albumartist, GENRE: f.value.genre, DATE: f.value.date,
    TRACKNUMBER: f.value.track, LYRICS: f.value.lyrics
  };
  const body = { clear: false, tags: Object.fromEntries(Object.entries(tags).filter(([, v]) => v !== '')) };
  try {
    const r = await api('/api/tracks/' + encodeURIComponent(state.currentId) + '/tags', { method: 'POST', body: JSON.stringify(body) });
    toast('标签已保存' + (r.renamed ? '，文件已重命名为：' + r.name : ''), 'ok');
    await refresh();
    if (r.renamed) loadTrack(state.tracks.find(t => t.id === state.currentId) || cur());
  } catch (e) { toast('保存失败: ' + e.message, 'err'); }
}

async function getLyrics() {
  if (!cur()) return;
  const title = f.value.title.trim(), artist = f.value.artist.trim();
  if (!title) { toast('请先填写标题', 'err'); return; }
  f.value.lyrics = '获取中…';
  const artists = artist ? artist.split('/').map(s => s.trim()).filter(Boolean) : [];
  try {
    const r = await api('/api/lyrics?source=auto&title=' + encodeURIComponent(title) + '&artists=' + encodeURIComponent(artists.join('||')));
    f.value.lyrics = r.lyric || '';
    if (!r.lyric) toast('未获取到歌词', 'err');
  } catch (e) { f.value.lyrics = ''; toast('获取歌词失败: ' + e.message, 'err'); }
}
function clearLyrics() { f.value.lyrics = ''; }

function fmtSize(s) {
  if (!s) return '';
  const mb = s / 1048576;
  return mb >= 1024 ? (mb / 1024).toFixed(1) + ' GB' : mb.toFixed(1) + ' MB';
}
function fmtDur(d) {
  if (!d) return '';
  const m = Math.floor(d / 60), s = Math.floor(d % 60);
  return m + ':' + String(s).padStart(2, '0');
}

// 导入同目录外挂 .lrc 歌词到标签
async function importLrc() {
  if (!cur()) return;
  try {
    const r = await api('/api/tracks/' + encodeURIComponent(state.currentId) + '/lrcfile');
    f.value.lyrics = r.content || '';
    toast('已导入外挂歌词：' + r.name, 'ok');
  } catch (e) { toast('读取外挂歌词失败: ' + e.message, 'err'); }
}

async function clearCover() {
  if (!cur()) return;
  try {
    await api('/api/tracks/' + encodeURIComponent(state.currentId) + '/cover', { method: 'POST', body: JSON.stringify({ clear: true }) });
    toast('封面已清除', 'ok');
    await refresh();
  } catch (e) { toast('失败: ' + e.message, 'err'); }
}

async function mbSearch() {
  const q = sQuery.value.trim();
  if (!q) { toast('请输入搜索关键词', 'err'); return; }
  searching.value = true;
  state.mbRes = [];
  try {
    const res = await api('/api/scrape/search?q=' + encodeURIComponent(q) + '&source=' + encodeURIComponent(sSource.value) + '&limit=8');
    state.mbRes = res || [];
  } catch (e) { toast('搜索失败: ' + e.message, 'err'); }
  searching.value = false;
}

async function apply(i, withCover) {
  const r = state.mbRes[i];
  if (!r || !cur()) return;
  const body = {
    source: r.source, sourceId: r.sourceId, title: r.title, artists: r.artists || [],
    album: r.album, albumArtist: r.albumArtist || [], genre: r.genre || [],
    trackNumber: r.trackNumber || 0, date: r.date,
    coverURL: r.coverURL, albumID: r.albumID || '',
    releaseMBID: r.releaseMBID || '', fetchCover: withCover,
    fields: selFields(withCover)
  };
  try {
    await api('/api/tracks/' + encodeURIComponent(state.currentId) + '/scrape', { method: 'POST', body: JSON.stringify(body) });
    toast('刮削完成', 'ok');
    await refresh();
  } catch (e) { toast('刮削失败: ' + e.message, 'err'); }
}

function srcLabel(name) { return (state.sources.find(s => s.name === name) || { label: name }).label; }
const t = cur;
</script>

<template>
  <div v-if="t()" class="form">
    <div class="row" style="margin-bottom:10px">
      <img class="cover-lg" :src="`/api/tracks/${encodeURIComponent(t().id)}/cover`" onerror="this.style.visibility='hidden'" />
      <div>
        <div style="font-weight:600">{{ t().tags.TITLE || t().fileName }}</div>
        <div class="muted" style="font-size:13px">{{ t().path }}</div>
        <div class="muted" style="font-size:12px">
          {{ fmtSize(t().size) }} · {{ fmtDur(t().duration) }} · {{ t().bitrate || '?' }} kbps · {{ t().sampleRate ? (t().sampleRate / 1000).toFixed(1) + ' kHz' : '' }} · SHA256: {{ t().sha256 ? t().sha256.slice(0, 20) + '…' : '—（打开去重页后自动计算）' }}
        </div>
      </div>
    </div>
    <label>标题 TITLE</label><input type="text" v-model="f.title" />
    <label>艺术家 ARTIST</label><input type="text" v-model="f.artist" />
    <label>专辑 ALBUM</label><input type="text" v-model="f.album" />
    <label>专辑艺术家 ALBUMARTIST</label><input type="text" v-model="f.albumartist" />
    <label>流派 GENRE</label><input type="text" v-model="f.genre" />
    <label>年份 DATE</label><input type="text" v-model="f.date" />
    <label>曲目号 TRACKNUMBER</label><input type="text" v-model="f.track" />

    <div style="display:flex;align-items:center;justify-content:space-between;margin-top:14px">
      <label style="margin:0">歌词 LYRICS</label>
      <div class="row" style="gap:6px">
        <button v-if="t().hasLrcFile" class="ghost sm" @click="importLrc" title="读取同目录 .lrc 文件内容">导入外挂 LRC</button>
        <button class="ghost sm" @click="getLyrics">获取歌词</button>
        <button class="ghost sm" @click="clearLyrics">清除歌词</button>
      </div>
    </div>
    <textarea rows="8" v-model="f.lyrics"></textarea>

    <div class="row" style="margin-top:12px">
      <button class="sm" @click="save">保存标签</button>
      <button class="ghost sm" @click="clearCover">清除封面</button>
    </div>

    <label style="margin-top:14px">🔍 刮削搜索（选择来源）</label>
    <div class="row">
      <select v-model="sSource" style="padding:7px 10px;border-radius:8px;border:1px solid var(--line);background:var(--panel2);color:var(--text);min-width:120px">
        <option v-for="s in state.sources" :key="s.name" :value="s.name">{{ s.label }}</option>
      </select>
      <input type="text" v-model="sQuery" placeholder="自动按标题+艺术家检索，可覆盖" @keydown.enter="mbSearch" />
      <button class="sm" @click="mbSearch">搜索</button>
    </div>

    <div v-if="searching" class="loading" style="margin-top:8px">搜索…</div>
    <div v-else-if="state.mbRes.length === 0" class="muted" style="margin-top:8px">尚无搜索结果</div>
    <div v-else>
      <label style="margin:10px 0 4px">应用字段（可选）</label>
      <div class="field-grid">
        <label class="chk"><input type="checkbox" class="tchk" v-model="sf.title" /> 歌名</label>
        <label class="chk"><input type="checkbox" class="tchk" v-model="sf.artist" /> 艺术家</label>
        <label class="chk"><input type="checkbox" class="tchk" v-model="sf.albumArtist" /> 专辑艺术家</label>
        <label class="chk"><input type="checkbox" class="tchk" v-model="sf.genre" /> 流派</label>
        <label class="chk"><input type="checkbox" class="tchk" v-model="sf.year" /> 年代</label>
        <label class="chk"><input type="checkbox" class="tchk" v-model="sf.trackNumber" /> 曲目号</label>
        <label class="chk"><input type="checkbox" class="tchk" v-model="sf.lyrics" /> 歌词</label>
        <label class="chk"><input type="checkbox" class="tchk" v-model="sfCover" /> 海报</label>
      </div>
    </div>
    <div v-for="(r, i) in state.mbRes" :key="i" class="res-item">
      <div class="row" style="gap:10px;align-items:flex-start">
        <img v-if="r.coverURL" class="thumb" :src="r.coverURL" onerror="this.style.visibility='hidden'" />
        <div style="flex:1;min-width:0">
          <div class="t"><span class="tag src">{{ srcLabel(r.source) }}</span> {{ r.title }} <span class="muted">{{ r.artists ? r.artists.join(', ') : '' }}</span></div>
          <div class="m">{{ r.album }} <span class="muted">({{ r.date || '未知年份' }})</span></div>
          <div class="acts">
            <button class="sm" @click="apply(i, true)">应用(含封面)</button>
            <button class="ghost sm" @click="apply(i, false)">仅标签</button>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div v-else class="muted">在左侧列表点击某一行曲目，在此编辑标签、获取歌词或刮削。</div>
</template>
