<script setup>
import { reactive, ref } from 'vue'
import { state, toast, refresh, startBatchPoll } from '../store.js'
import { api } from '../api.js'
import FormatDialog from './FormatDialog.vue'

const skipFilled = ref(true)
const fmtOpen = ref(false)
const idBusy = ref(false)
const idResult = ref(null)

// 简繁转换
const scriptTo = ref('trad')

function needsSel() { if (state.selected.size === 0) { toast('请先在左侧列表勾选曲目', 'err'); return false; } return true; }

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

async function setChorus() {
  if (!needsSel()) return;
  try {
    const r = await api('/api/set-chorus', {
      method: 'POST', body: JSON.stringify({ ids: [...state.selected] })
    });
    const n = (r.results || []).filter(x => x.changed).length;
    toast('合唱转换完成：' + n + '/' + r.total + ' 首（艺术家>3 位才改）', n ? 'ok' : 'err');
    clearSel();
    await refresh();
  } catch (e) { toast('操作失败: ' + e.message, 'err'); }
}

// 音频内容识别：无标签/无文件名信息的曲目（track01.mp3 这类），按音频指纹
// 与库内已标注曲目匹配，命中后写入标签并抓取歌词
async function identify() {
  if (state.selected.size === 0) { toast('请先勾选要识别的曲目（可在列表筛选「待识别」）', 'err'); return; }
  idBusy.value = true;
  idResult.value = null;
  try {
    const r = await api('/api/identify', {
      method: 'POST', body: JSON.stringify({ ids: [...state.selected] })
    });
    const rs = r.results || [];
    const matched = rs.filter(x => x.ok).length;
    idResult.value = { matched, total: rs.length };
    toast('音频识别完成：' + matched + '/' + rs.length + ' 首已识别', matched ? 'ok' : 'err');
    clearSel();
    await refresh();
  } catch (e) { toast('音频识别失败: ' + e.message, 'err'); }
  finally { idBusy.value = false; }
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

async function startBatch() {
  if (state.selected.size === 0) { toast('请先在列表勾选曲目', 'err'); return; }
  const names = selectedFieldNames();
  if (names.length === 0) { toast('请至少勾选一个要补全的字段', 'err'); return; }
  state.batchBusy = true;
  state.batchJob = null;
  try {
    const r = await api('/api/scrape/batch', {
      method: 'POST',
      body: JSON.stringify({
        ids: [...state.selected], source: state.bSource,
        fields: names, skipFilled: skipFilled.value
      })
    });
    // 全局轮询：后台运行、任意页面可见进度，并增量合并列表（不整表刷新）
    startBatchPoll(r.jobId);
  } catch (e) {
    toast('批量刮削启动失败: ' + e.message, 'err');
    state.batchBusy = false;
  }
}
</script>

<template>
  <div class="muted" style="margin-bottom:10px">勾选左侧列表中的曲目（也可全选），自动从所选源补全下方勾选的字段；已含该标签的曲目自动跳过，节省硬件开销。</div>

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
  </div>

  <div v-if="state.batchJob" style="margin-top:12px">
    <div class="progbar"><div class="progfill" :style="{ width: (state.batchJob.total ? Math.round(state.batchJob.done / state.batchJob.total * 100) : 0) + '%' }"></div></div>
    <div class="muted">
      {{ state.batchJob.status === 'done'
        ? ('完成：' + state.batchJob.results.filter(r => r.ok).length + '/' + state.batchJob.total + ' 成功')
        : ('进度 ' + state.batchJob.done + '/' + state.batchJob.total + ' · ' + (state.batchJob.current || '处理中…')) }}
    </div>
    <div v-if="state.batchJob.status === 'done' && state.batchJob.results.some(r => !r.ok && r.message)" class="err" style="white-space:pre-wrap">
      <div v-for="(r, i) in state.batchJob.results.filter(x => !x.ok && x.message)" :key="i">{{ r.fileName }}: {{ r.message }}</div>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">🔊 音频内容识别</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" :disabled="idBusy" @click="identify">{{ idBusy ? '识别中…' : '识别勾选的 ' + state.selected.size + ' 首' }}</button>
      <span class="muted">无标签、无文件名信息的曲目（如 track01.mp3），按音频内容与库内已标注曲目匹配，命中即补全标签与歌词</span>
    </div>
    <div v-if="idResult" class="muted" style="margin-top:6px">已识别 {{ idResult.matched }} / {{ idResult.total }} 首（识别成功即写入标签）</div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">🔁 格式转换（ffmpeg 转码）</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" @click="fmtOpen = true">格式转换…</button>
      <span class="muted">批量转换音频格式、比特率和采样率，保存在同目录，不会删除源文件</span>
    </div>
  </div>
  <FormatDialog :open="fmtOpen" @close="fmtOpen = false" />

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">🧹 乱码修复</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" @click="fixEnc">修复勾选 {{ state.selected.size }} 首的乱码标签</button>
      <span class="muted">自动识别并还原 GBK/UTF-8 错读的中文标题、艺术家等字段</span>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">🎤 艺术家处理</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" @click="setChorus">艺术家>3 位 → 改为“合唱”</button>
      <span class="muted">按 / ; 、 ， _ 拆分艺术家，超过 3 位时统一改为“合唱”</span>
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
