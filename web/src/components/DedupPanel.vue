<script setup>
import { reactive, watch } from 'vue'
import { state, toast, loadDups, refresh } from '../store.js'
import { api } from '../api.js'

function fileName(id) { const t = state.tracks.find(x => x.id === id); return t ? t.fileName : id; }

const methodMeta = {
  hash: { cls: 'hash', label: 'SHA256' },
  fingerprint: { cls: 'fp', label: '指纹' },
  format: { cls: 'src', label: '格式' },
  tags: { cls: 'src', label: '艺术家+标题' }
};
function meta(g) { return methodMeta[g.method] || methodMeta.hash; }

// 每个分组的多选状态（默认勾选除“保留最佳”外的全部，用户可自行调整）
const checks = reactive({})
watch(() => state.dupGroups, (groups) => {
  groups.forEach((g, gi) => {
    if (!checks[gi]) checks[gi] = new Set();
    const keep = g.keepId || g.ids[0];
    g.ids.forEach(id => { if (id !== keep) checks[gi].add(id); });
  });
}, { immediate: true });

function tog(gi, id) {
  if (checks[gi].has(id)) checks[gi].delete(id); else checks[gi].add(id);
}
function selIds(gi, g) { return g.ids.filter(id => checks[gi] && checks[gi].has(id)); }

async function delChecked(gi, g) {
  const del = selIds(gi, g);
  if (!del.length) { toast('请先勾选要删除的项', 'err'); return; }
  const sure = confirm('将删除 ' + del.length + ' 个文件（不会删除勾选外的保留项）：\n' + del.map(fileName).join('\n') + '\n\n确定删除吗？');
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
    重复分组：SHA256 / 指纹 / 同名不同格式 / 艺术家+标题。勾选要删除的项（默认保留最佳音质），再点「删除选中」。
  </div>
  <div v-if="state.dupGroups.length === 0" class="muted" style="margin-top:10px">未发现重复。</div>
  <div v-for="(g, gi) in state.dupGroups" :key="gi" class="dup">
    <span class="tag" :class="meta(g).cls">{{ meta(g).label }}</span>
    <span class="dupcol">
      <span class="duphead"><b>{{ g.count }}</b> 首 · {{ g.key }}</span>
      <span v-for="id in g.ids" :key="id" class="dupitem">
        <input type="checkbox" class="tchk" :checked="checks[gi] && checks[gi].has(id)" @change="tog(gi, id)" />
        <span v-if="g.keepId === id" class="tag src">建议保留</span>
        {{ fileName(id) }}
      </span>
      <button class="ghost sm" style="margin-top:4px;align-self:flex-start" @click="delChecked(gi, g)">删除选中 ({{ selIds(gi, g).length }})</button>
    </span>
  </div>
</template>
