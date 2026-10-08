<script setup>
import { reactive, watch } from 'vue'
import { state, toast, loadDups, refresh } from '../store.js'
import { api } from '../api.js'

function fileName(id) { const t = state.tracks.find(x => x.id === id); return t ? t.fileName : id; }

// 每个“格式”组的保留选择（默认建议最佳）
const keepSel = reactive({})
watch(() => state.dupGroups, (groups) => {
  groups.forEach((g, i) => {
    if (g.method === 'format' && keepSel[i] === undefined) {
      keepSel[i] = g.keepId || g.ids[0];
    }
  });
}, { immediate: true });

async function keepGroup(g, i) {
  const chosen = keepSel[i];
  if (!chosen) { toast('请先选择要保留的那一首', 'err'); return; }
  const del = g.ids.filter(id => id !== chosen);
  if (del.length === 0) return;
  const sure = confirm('将删除以下 ' + del.length + ' 个文件（保留所选的最佳音质）：\n' + del.map(fileName).join('\n') + '\n\n确定删除吗？');
  if (!sure) return;
  try {
    const r = await api('/api/duplicates/remove', { method: 'POST', body: JSON.stringify({ ids: del }) });
    toast('已删除 ' + r.removed + ' 个文件' + (r.failed.length ? '，失败 ' + r.failed.length + ' 个' : ''), r.failed.length ? 'err' : 'ok');
    await refresh();
    await loadDups();
  } catch (e) { toast('删除失败: ' + e.message, 'err'); }
}
</script>

<template>
  <button class="ghost sm" @click="loadDups">重新检测</button>
  <div class="muted" style="margin-top:8px;font-size:12px">
    同名不同格式：只做检测、不自动删除，请在下单选好保留的最佳音质后手动删除其余。
  </div>
  <div v-if="state.dupGroups.length === 0" class="muted" style="margin-top:10px">未发现重复。</div>
  <div v-for="(g, gi) in state.dupGroups" :key="gi" class="dup">
    <span class="tag" :class="g.method === 'hash' ? 'hash' : (g.method === 'format' ? 'src' : 'fp')">{{ g.method === 'hash' ? 'SHA256' : (g.method === 'format' ? '格式' : '指纹') }}</span>
    <span class="dupcol">
      <span class="duphead"><b>{{ g.count }}</b> 首 · 同名不同格式（按音质评分）</span>
      <span v-for="id in g.ids" :key="id" class="dupitem">
        <input type="radio" v-if="g.method === 'format'" :name="'keep-' + gi" :value="id" v-model="keepSel[gi]" />
        <span v-if="g.method === 'format' && keepSel[gi] === id" class="tag src">保留</span>
        <span v-if="g.method === 'format' && g.keepId === id && keepSel[gi] !== id" class="muted" style="font-size:11px">（建议最佳）</span>
        {{ fileName(id) }}
      </span>
      <button v-if="g.method === 'format'" class="ghost sm" style="margin-top:4px" @click="keepGroup(g, gi)">保留所选，删除其余 {{ g.count - 1 }} 首</button>
    </span>
  </div>
</template>
