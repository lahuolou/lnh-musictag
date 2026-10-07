<script setup>
import { state, loadDups } from '../store.js'

function fileName(id) { const t = state.tracks.find(x => x.id === id); return t ? t.fileName : id; }
</script>

<template>
  <button class="ghost sm" @click="loadDups">重新检测</button>
  <div v-if="state.dupGroups.length === 0" class="muted" style="margin-top:10px">未发现重复。</div>
  <div v-for="(g, gi) in state.dupGroups" :key="gi" class="dup">
    <span class="tag" :class="g.method === 'hash' ? 'hash' : 'fp'">{{ g.method === 'hash' ? 'SHA256' : '指纹' }}</span>
    <span><b>{{ g.count }}</b> 首 · {{ g.ids.map(fileName).join(' / ') }}</span>
  </div>
</template>
