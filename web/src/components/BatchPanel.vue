<script setup>
import { reactive, ref } from 'vue'
import { state, toast, refresh, startBatchPoll, startToolJob, stopToolJob } from '../store.js'
import { api } from '../api.js'
import { t } from '../i18n.js'
import FormatDialog from './FormatDialog.vue'

const skipFilled = ref(true)
const fmtOpen = ref(false)

// 简繁转换（默认繁体 → 简体，常见于整理港台唱片）
const scriptTo = ref('simp')

function needsSel() { if (state.selected.size === 0) { toast(t('selFirst'), 'err'); return false; } return true; }

// 通用后台任务：音频识别 / 乱码修复 / 简繁转换 / 格式转换共用。
// 启动 job 后轮询进度，完成时 toast + 清勾选 + 静默刷新列表。
const doneKeys = { identify: 'identifyDone', fixEnc: 'fixMojiDone', script: 'scriptDone', convert: 'fmtDone' }
const failKeys = { identify: 'identifyFail', fixEnc: 'fixMojiFail', script: 'scriptFail', convert: 'fmtFail' }

async function runToolJob(kind, url, body) {
  if (!needsSel()) return;
  stopToolJob();
  try {
    const r = await api(url, { method: 'POST', body: JSON.stringify(body) });
    if (!r.jobId) { // 无目标（如识别：库内无待识别曲目）
      toast(t(doneKeys[kind], { ok: 0, t: 0 }), 'err');
      return;
    }
    await startToolJob(r.jobId, r.total);
    // 等待任务完成（startToolJob 立即返回，完成信号通过状态轮询到位）
    const iv = setInterval(() => {
      const tj = state.toolJob;
      if (tj && tj.finished) {
        clearInterval(iv);
        toast(t(doneKeys[kind], { ok: tj.ok, t: tj.total }), tj.ok === tj.total ? 'ok' : 'err');
        state.selected.clear();
        refresh();
        if (kind === 'convert') fmtOpen.value = false;
      } else if (!tj) {
        clearInterval(iv);
      }
    }, 500);
  } catch (e) { toast(t(failKeys[kind], { m: e.message }), 'err'); }
}

async function fixEnc() { await runToolJob('fixEnc', '/api/fix-encoding', { ids: [...state.selected] }); }
async function convertScript() { await runToolJob('script', '/api/convert-script', { ids: [...state.selected], to: scriptTo.value }); }

async function setChorus() {
  if (!needsSel()) return;
  try {
    const r = await api('/api/set-chorus', {
      method: 'POST', body: JSON.stringify({ ids: [...state.selected] })
    });
    const n = (r.results || []).filter(x => x.changed).length;
    toast(t('chorusDone', { n, t: r.total }), n ? 'ok' : 'err');
    clearSel();
    await refresh();
  } catch (e) { toast(t('delFail', { m: e.message }), 'err'); }
}

// 音频内容识别：无标签/无文件名信息的曲目（track01.mp3 这类），按音频指纹
// 与库内已标注曲目匹配，命中后写入标签并抓取歌词
async function identify() {
  if (state.selected.size === 0) { toast(t('identifySelFirst'), 'err'); return; }
  await runToolJob('identify', '/api/identify', { ids: [...state.selected] });
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
  if (state.selected.size === 0) { toast(t('selFirst'), 'err'); return; }
  const names = selectedFieldNames();
  if (names.length === 0) { toast(t('batchNoField'), 'err'); return; }
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
    toast(t('batchStartFail', { m: e.message }), 'err');
    state.batchBusy = false;
  }
}
</script>

<template>
  <div class="muted" style="margin-bottom:10px">{{ t('batchHint') }}</div>

  <div class="panel" style="margin-top:0">
    <div class="field-grid">
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.cover" /> {{ t('bCover') }}</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.title" /> {{ t('bTitle') }}</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.artist" /> {{ t('bArtist') }}</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.albumArtist" /> {{ t('bAlbumArtist') }}</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.genre" /> {{ t('bGenre') }}</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.year" /> {{ t('bYear') }}</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.trackNumber" /> {{ t('bTrack') }}</label>
      <label class="chk"><input type="checkbox" class="tchk" v-model="fields.lyrics" /> {{ t('bLyrics') }}</label>
    </div>
    <label class="chk" style="margin-top:8px">
      <input type="checkbox" class="tchk" v-model="skipFilled" />
      {{ t('smartSkip') }}
    </label>
  </div>

  <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center">
    <button class="ghost sm" @click="clearSel">{{ t('clearSel') }}</button>
    <select v-model="state.bSource" style="padding:7px 10px;border-radius:8px;border:1px solid var(--line);background:var(--panel2);color:var(--text)">
      <option v-for="s in state.sources" :key="s.name" :value="s.name">{{ s.label }}</option>
    </select>
    <button :disabled="state.batchBusy" @click="startBatch">{{ t('batchRun', { n: state.selected.size }) }}</button>
  </div>

  <div v-if="state.batchJob" style="margin-top:12px">
    <div class="progbar"><div class="progfill" :style="{ width: (state.batchJob.total ? Math.round(state.batchJob.done / state.batchJob.total * 100) : 0) + '%' }"></div></div>
    <div class="muted">
      {{ state.batchJob.status === 'done'
        ? t('batchDone', { ok: state.batchJob.results.filter(r => r.ok).length, t: state.batchJob.total })
        : t('batchProgressBar', { d: state.batchJob.done, t: state.batchJob.total, c: state.batchJob.current || t('processing') }) }}
    </div>
    <div v-if="state.batchJob.status === 'done' && state.batchJob.results.some(r => !r.ok && r.message)" class="err" style="white-space:pre-wrap">
      <div v-for="(r, i) in state.batchJob.results.filter(x => !x.ok && x.message)" :key="i">{{ r.fileName }}: {{ r.message }}</div>
    </div>
  </div>

  <div v-if="state.toolJob" style="margin-top:12px">
    <div class="progbar"><div class="progfill" :style="{ width: (state.toolJob.total ? Math.round(state.toolJob.done / state.toolJob.total * 100) : 0) + '%' }"></div></div>
    <div class="muted">
      {{ state.toolJob.status === 'done'
        ? t('jobDone', { ok: state.toolJob.ok, t: state.toolJob.total })
        : t('jobRunning', { d: state.toolJob.done, t: state.toolJob.total, c: state.toolJob.current || t('processing') }) }}
    </div>
    <div v-if="state.toolJob.status === 'done' && state.toolJob.errors.length" class="err" style="white-space:pre-wrap">
      <div v-for="(r, i) in state.toolJob.errors" :key="i">{{ r.fileName }}: {{ r.message }}</div>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">{{ t('identify') }}</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" :disabled="state.toolJob && state.toolJob.status === 'running'" @click="identify">{{ state.toolJob && state.toolJob.status === 'running' ? t('identifying') : t('identifyRun', { n: state.selected.size }) }}</button>
      <span class="muted">{{ t('identifyHint') }}</span>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">{{ t('fmtConvert') }}</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" @click="fmtOpen = true">{{ t('fmtRun') }}</button>
      <span class="muted">{{ t('fmtHint') }}</span>
    </div>
  </div>
  <FormatDialog :open="fmtOpen" @close="fmtOpen = false" />

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">{{ t('fixMoji') }}</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" @click="fixEnc">{{ t('fixMojiRun', { n: state.selected.size }) }}</button>
      <span class="muted">{{ t('fixMojiHint') }}</span>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">{{ t('chorus') }}</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <button class="ghost sm" @click="setChorus">{{ t('chorusRun') }}</button>
      <span class="muted">{{ t('chorusHint') }}</span>
    </div>
  </div>

  <div style="border-top:1px solid var(--line);margin-top:16px;padding-top:12px">
    <div class="h3">{{ t('scriptConv') }}</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:6px">
      <select v-model="scriptTo" style="padding:7px 10px;border-radius:8px;border:1px solid var(--line);background:var(--panel2);color:var(--text)">
        <option value="trad">{{ t('scriptToTrad') }}</option>
        <option value="simp">{{ t('scriptToSimp') }}</option>
      </select>
      <button class="ghost sm" @click="convertScript">{{ t('scriptRun', { n: state.selected.size }) }}</button>
    </div>
  </div>
</template>
