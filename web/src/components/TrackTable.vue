<script setup>
import { state } from '../store.js'

function toggleSel(id, ev) {
  if (ev.target.checked) state.selected.add(id);
  else state.selected.delete(id);
}
function toggleAll(ev) {
  const on = ev.target.checked;
  state.tracks.forEach(t => { if (on) state.selected.add(t.id); else state.selected.delete(t.id); });
}
function openEdit(t) {
  state.currentId = t.id;
  state.route = '/edit/' + t.id;
  location.hash = '/edit/' + t.id;
}
function fmtDur(sec) {
  if (!sec) return '--';
  const m = Math.floor(sec / 60), s = Math.round(sec % 60);
  return m + ':' + String(s).padStart(2, '0');
}
function artists(t) { return (t.tags.ARTIST || '').split(' / ').join(', '); }
const allChecked = () => state.tracks.length > 0 && state.selected.size === state.tracks.length;
</script>

<template>
  <table>
    <thead>
      <tr>
        <th style="width:32px"><input type="checkbox" class="tchk" :checked="allChecked()" @change="toggleAll" /></th>
        <th style="width:52px">封面</th><th>标题</th><th>艺术家</th><th>专辑</th><th style="width:60px">时长</th><th style="width:50px"></th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="t in state.tracks" :key="t.id" style="cursor:pointer" @click="openEdit(t)">
        <td style="text-align:center" @click.stop><input type="checkbox" class="tchk" :checked="state.selected.has(t.id)" @change="toggleSel(t.id, $event)" /></td>
        <td><img class="thumb" :src="`/api/tracks/${encodeURIComponent(t.id)}/cover`" onerror="this.style.visibility='hidden'" /></td>
        <td>{{ t.tags.TITLE || t.fileName }}</td>
        <td>{{ artists(t) }}</td>
        <td>{{ t.tags.ALBUM }}</td>
        <td class="muted">{{ fmtDur(t.duration) }}</td>
        <td><span v-if="t.fingerprint" class="tag fp">指纹</span></td>
      </tr>
    </tbody>
  </table>
</template>
