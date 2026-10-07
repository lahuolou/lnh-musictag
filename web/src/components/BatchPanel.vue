<script setup>
import { ref } from 'vue'
import { state, toast, refresh } from '../store.js'
import { api } from '../api.js'

const prog = ref(null)

function clearSel() { state.selected.clear(); }

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
  state.batchBusy = true;
  prog.value = null;
  try {
    const r = await api('/api/scrape/batch', {
      method: 'POST',
      body: JSON.stringify({
        ids: [...state.selected], fetchCover: state.bFetchCover,
        fetchLyrics: state.bFetchLyrics, source: state.bSource
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
        toast('批量刮削完成：' + ok + '/' + j.total + ' 成功', ok === j.total ? 'ok' : 'err');
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
  <div class="muted" style="margin-bottom:10px">勾选②列表中的曲目（也可全选），自动从所选源刮削并写入标签。</div>
  <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:center">
    <button class="ghost sm" @click="clearSel">清空选择</button>
    <label class="chk"><input type="checkbox" v-model="state.bFetchCover" /> 含封面</label>
    <label class="chk"><input type="checkbox" v-model="state.bFetchLyrics" /> 含歌词</label>
    <select v-model="state.bSource" style="padding:7px 10px;border-radius:8px;border:1px solid var(--line);background:var(--panel2);color:var(--text)">
      <option v-for="s in state.sources" :key="s.name" :value="s.name">{{ s.label }}</option>
    </select>
    <button :disabled="state.batchBusy" @click="startBatch">刮削选中 ({{ state.selected.size }})</button>
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
</template>
