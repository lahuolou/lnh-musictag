<script setup>
import { reactive, watch } from 'vue'
import { state, toast, loadDups, refresh } from '../store.js'
import { api } from '../api.js'

function track(id) { return state.tracks.find(x => x.id === id); }
function fileName(id) { const t = track(id); return t ? t.fileName : id; }
function fileInfo(id) {
  const t = track(id);
  if (!t) return '';
  const mb = t.size ? (t.size / 1048576).toFixed(1) + ' MB' : '';
  const dur = t.duration ? Math.floor(t.duration / 60) + ':' + String(Math.floor(t.duration % 60)).padStart(2, '0') : '';
  return [mb, dur, t.bitrate ? t.bitrate + ' kbps' : ''].filter(Boolean).join(' · ');
}

const methodMeta = {
  hash: { cls: 'hash', label: 'SHA256（内容相同）' },
  format: { cls: 'src', label: '格式（同名多格式）' },
  tags: { cls: 'src', label: '艺术家+标题（同歌不同名）' }
};
function meta(g) { return methodMeta[g.method] || methodMeta.hash; }

// 每个分组的多选状态：默认不勾选，由用户自行选择要删除的项（更稳妥）
const checks = reactive({})
watch(() => state.dupGroups, (groups) => {
  groups.forEach((g, gi) => { if (!checks[gi]) checks[gi] = new Set(); });
}, { immediate: true });

function tog(gi, id) {
  if (checks[gi].has(id)) checks[gi].delete(id); else checks[gi].add(id);
}
function selIds(gi, g) { return g.ids.filter(id => checks[gi] && checks[gi].has(id)); }
function checkAll(gi, g) {
  // 全选：勾选除“建议保留”外的全部
  const keep = g.keepId || g.ids[0];
  g.ids.forEach(id => { if (id !== keep) checks[gi].add(id); });
}
function clearGroup(gi, g) { checks[gi].clear(); }

async function delChecked(gi, g) {
  const del = selIds(gi, g);
  if (!del.length) { toast('请先勾选要删除的项', 'err'); return; }
  const sure = confirm('将删除 ' + del.length + ' 个文件（勾选外的保留）：\n' + del.map(fileName).join('\n') + '\n\n确定删除吗？此操作不可撤销。');
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
  <div class="muted" style="margin-top:8px;font-size:12px;line-height:1.7">
    重复分组：SHA256 内容相同 / 指纹同录音 / 同名不同格式 / 艺术家+标题相同。<br />
    先勾选要删除的项（默认不勾选，可“全选”后取消保留项），再点「删除选中」。
  </div>
  <div v-if="state.dupGroups.length === 0" class="muted" style="margin-top:10px">未发现重复。</div>
  <div v-for="(g, gi) in state.dupGroups" :key="gi" class="dup">
    <span class="tag" :class="meta(g).cls">{{ meta(g).label }}</span>
    <span class="dupcol">
      <span class="duphead"><b>{{ g.count }}</b> 首 · {{ g.key }}</span>
      <span v-for="id in g.ids" :key="id" class="dupitem" :title="fileInfo(id)">
        <input type="checkbox" class="tchk" :checked="checks[gi] && checks[gi].has(id)" @change="tog(gi, id)" />
        <span v-if="g.keepId === id" class="tag src">建议保留</span>
        {{ fileName(id) }}
        <span class="muted" style="margin-left:auto;font-size:11px">{{ fileInfo(id) }}</span>
      </span>
      <span style="display:flex;gap:8px;margin-top:4px">
        <button class="ghost sm" @click="checkAll(gi, g)">全选（保留最佳除外）</button>
        <button class="ghost sm" @click="clearGroup(gi, g)">清空</button>
        <button class="ghost sm" :disabled="!selIds(gi, g).length" @click="delChecked(gi, g)">删除选中 ({{ selIds(gi, g).length }})</button>
      </span>
    </span>
  </div>
</template>
