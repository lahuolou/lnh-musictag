<script setup>
import { state, loadDups } from '../store.js'

function fileName(id) { const t = state.tracks.find(x => x.id === id); return t ? t.fileName : id; }
</script>

<template>
  <button class="ghost sm" @click="loadDups">重新检测</button>
  <div v-if="state.dupGroups.length === 0" class="muted" style="margin-top:10px">未发现重复。</div>
  <div v-for="(g, gi) in state.dupGroups" :key="gi" class="dup">
    <span class="tag" :class="g.method === 'hash' ? 'hash' : (g.method === 'format' ? 'src' : 'fp')">{{ g.method === 'hash' ? 'SHA256' : (g.method === 'format' ? '格式' : '指纹') }}</span>
    <span class="dupcol">
      <span class="duphead"><b>{{ g.count }}</b> 首
        <span v-if="g.method === 'format'">同名不同格式，保留音质最佳</span>
      </span>
      <span v-for="id in g.ids" :key="id" class="dupitem">
        <span v-if="g.keepId === id" class="tag src">保留(最佳)</span>
        <span v-else class="muted tagx">可删</span>
        {{ fileName(id) }}
      </span>
    </span>
  </div>
</template>
