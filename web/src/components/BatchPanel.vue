<script setup>
import { reactive, ref } from 'vue'
import { state, toast, refresh } from '../store.js'
import { api } from '../api.js'

const prog = ref(null)
const skipFilled = ref(true)

// 格式转换
const convertTarget = ref('flac')
const convertRemove = ref(false)
const convertBusy = ref(false)
const convertFormats = ['mp3', 'flac', 'm4a', 'ogg', 'opus', 'wav']
// 简繁转换
const scriptTo = ref('trad')

function needsSel() { if (state.selected.size === 0) { toast('请先在列表勾选曲目', 'err'); return false; } return true; }

async function convert() {
  if (!needsSel()) return;
  convertBusy.value = true;
  try {
    const r = await api('/api/convert', {
      method: 'POST',
      body: JSON.stringify({ ids: [...state.selected], target: convertTarget.value, removeSource: convertRemove.value })
    });
    const ok = (r.results || []).filter(x => x.ok).length;
    toast('格式转换完成：' + ok + '/' + r.total + ' 首', ok === r.total ? 'ok' : 'err');
    clearSel();
    await refresh();
  } catch (e) { toast('格式转换失败: ' + e.message, 'err'); }
  finally { convertBusy.value = false; }
}

async function fixEnc() {
  if (!needsSel()) return;
  try {
    const r = await api('/api/fix-encoding', {
      method: 'POST', body: JSON.stringify({ ids: [...state.selected] })
    });
    const n = (r.results || []).reduce((a, x) => a + Object.keys(x.changed || {}).length, 0);
    toast('乱码修复完成：' + n + ' 个字段已修复', 'ok');
    clearSel();
    await refresh();
  } catch (e) { toast('乱码修复失败: ' + e.message, 'err'); }
}

async function convertScript() {
  if (!needsSel()) return;
  try {
    const r = await api('/api/convert-script', {
      method: 'POST', body: JSON.stringify({ ids: [...state.selected], to: scriptTo.value })
    });
    const n = (r.results || []).reduce((a, x) => a + x.changed, 0);
    toast('简繁转换完成：' + n + ' 个字段已转换', 'ok');
    clearSel();
    await refresh();
  } catch (e) { toast('简繁转换失败: ' + e.message, 'err'); }
}

// 批量补全可选字段（默认全选；已有该标签时智能跳过）
const fields = reactive({
  cover: true, title: true, artist: true, albumArtist: true,
  genre: true, year: true, trackNumber: true, lyrics: true
})

function clearSel() { state.selected.clear(); }

function selectedFieldNames() {
  const names = [];
  const m = { cover: 'cover', title: 'title', artist: 'artist', albumArtist: 'albumArtist', genre: 'genre', year: 'year', trackNumber: 'trackNumber', lyrics: 'lyrics' };
  for (const k in m) { if (fields[k]) names.push(m[k]); }
  return names;
}

async function fixTitle() {
  if (state.selected.size === 0) { toast('请先在列表勾选曲目', 'err'); return; }
  try {
    const r = await api('/api/tracks/fix-title', { method: 'POST', body: JSON.stringify({ ids: [...state.selected] }) });
    const arr = r.fixed || [];
    const n = arr.filter(x => x.fixed).length;
    toast('标题修正完成：' + n + '/' + arr.length + ' 首', 'ok');
    clearSel();
    await refresh();
  } catch (e) { toast('标题修正失败: ' + e.message, 'err'); }
}

async function startBatch() {
  if (state.selected.size === 0) { toast('请先在列表勾选曲目', 'err'); return; }
  const names = selectedFieldNames();
  if (names.length === 0) { toast('请至少勾选一个要补全的字段', 'err'); return; }
  state.batchBusy = true;
  prog.value = null;
  try {
    const r = await api('/api/scrape/batch', {
      method: 'POST',
      body: JSON.stringify({
        ids: [...state.selected], source: state.bSource,
        fields: names, skipFilled: skipFilled.value
      })
    });
    showProgress(r.jobId);
  } catch (e) {
    toast('批量刮削启动失败: ' + e.message, 'err');
    state.batchBusy = false;
  }
}

function showProgress(jobId) {
  const tick = async () => {
    try {
      const j = await api('/api/scrape/jobs/' + encodeURIComponent(jobId));
      prog.value = j;
      if (j.status === 'done') {
        state.batchBusy = false;
        const ok = (j.results || []).filter(r => r.ok).length;
        const skipped = (j.results || []).filter(r => r.ok && /智能跳过/.test(r.message || '')).length;
        toast('批量刮削完成：' + ok + '/' + j.total + ' 成功' + (skipped ? '（智能跳过 ' + skipped + '）' : ''), ok === j.total ? 'ok' : 'err');
        state.selected.clear();
        await refresh();
        return;
      }
      setTimeout(tick, 1000);
    } catch (e) {
      toast('查询进度失败: ' + e.message, 'err');
      state.batchBusy = false;
    }
  };
  tick();
}
</script>

<template>
  <div class="muted" style="margin-bottom:10px">勾选②列表中的曲目（也可全选），自动从所选源补全下方勾选的字段；已含该标签的曲目自动跳过，节省硬件开销。</div>

  <div class="panel" style="margin-top:0">
    <div class="field-grid">
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.cover" /> 海报</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.title" /> 歌名</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.artist" /> 艺术家</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.albumArtist" /> 专辑艺术家</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.genre" /> 流派</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.year" /> 年代</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.trackNumber" /> 曲目号</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.lyrics" /> 歌词</label>
    </div>
    <label class="chk" style="margin-top:8px">
      <input type="checkbox" class="tchk" v-model="skipFilled" />
      智能跳过：已有对应标签的曲目不再重复刮削
    </label>
  </div>

  <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center">
    <button class="ghost sm" @click="clearSel">清空选择</button>
    <select v-model="state.bSource" style="padding:7px 10px;border-radius:8px;border:1px solid var(--line);background:var(--panel2);color:var(--text)">
      <option v-for="s in state.sources" :key="s.name" :value="s.name">{{ s.label }}</option>
    </select>
    <button :disabled="state.batchBusy" @click="startBatch">批量补全 ({{ state.selected.size }})</button>
    <button class="ghost sm" @click="fixTitle">修正标题(去后缀)</button>
  </div>

  <div v-if="prog" style="margin-top:12px">
    <div class="progbar"><div class="progfill" :style="{ width: (prog.total ? Math.round(prog.done / prog.total * 100) : 0) + '%' }"></div></div>
    <div class="muted">
      {{ prog.status === 'done'
        ? ('完成：' + prog.results.filter(r => r.ok).length + '/' + prog.total + ' 成功')
        : ('进度 ' + prog.done + '/' + prog.total + ' · ' + (prog.current || '处理中…')) }}
    </div>
    <div v-if="prog.status === 'done' && prog.results.some(r => !r.ok && r.message)" class="err" style="white-space:pre-wrap">
      <div v-for="(r, i) in prog.results.filter(x => !x.ok && x.message)" :key="i">{{ r.fileName }}: {{ r.message }}</div>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">🔁 格式转换（ffmpeg 转码）</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <select v-model="convertTarget" style="padding:7px 10px;border-radius:8px;border:1px solid var(--line);background:var(--panel2);color:var(--text)">
        <option v-for="f in convertFormats" :key="f" :value="f">.{{ f }}</option>
      </select>
      <label class="chk"><input type="checkbox" class="tchk" v-model="convertRemove" /> 转换后删除原文件</label>
      <button class="ghost sm" :disabled="convertBusy" @click="convert">转换勾选的 {{ state.selected.size }} 首</button>
    </div>
    <div class="muted" style="margin-top:4px">转码为 .{{ convertTarget }}（保留标签元数据）；不勾删除则原文件保留、新增一首。</div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">🧹 乱码修复</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" @click="fixEnc">修复勾选 {{ state.selected.size }} 首的乱码标签</button>
      <span class="muted">自动识别并还原 GBK/UTF-8 错读的中文标题、艺术家等字段</span>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">🈶 简繁体转换</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <select v-model="scriptTo" style="padding:7px 10px;border-radius:8px;border:1px solid var(--line);background:var(--panel2);color:var(--text)">
        <option value="trad">简体 → 繁体</option>
        <option value="simp">繁体 → 简体</option>
      </select>
      <button class="ghost sm" @click="convertScript">转换勾选的 {{ state.selected.size }} 首</button>
    </div>
  </div>
</template>
