<script setup>
import { computed, onMounted } from 'vue'
import { state, toast, logout, go, refresh } from '../store.js'
import { api } from '../api.js'
import TrackTable from '../components/TrackTable.vue'
import EditPanel from '../components/EditPanel.vue'
import DedupPanel from '../components/DedupPanel.vue'
import BatchPanel from '../components/BatchPanel.vue'

const totalMin = computed(() => Math.round(state.tracks.reduce((a, t) => a + (t.duration || 0), 0) / 60))

async function scan() {
  const dir = state.scanDir.trim();
  if (!dir) { toast('请输入目录路径', 'err'); return; }
  state.scanMsg = '扫描中…';
  try {
    const r = await api('/api/scan', { method: 'POST', body: JSON.stringify({ dir }) });
    toast('新增 ' + r.added + ' 首，共 ' + r.total + ' 首', 'ok');
    await refresh();
  } catch (e) { toast('扫描失败: ' + e.message, 'err'); }
  state.scanMsg = '';
}

onMounted(async () => {
  try { state.sources = await api('/api/scrape/sources'); }
  catch (e) {
    state.sources = [
      { name: 'auto', label: '自动' }, { name: 'musicbrainz', label: 'MusicBrainz' },
      { name: 'itunes', label: 'iTunes' }, { name: 'netease', label: '网易云' }, { name: 'qq', label: 'QQ音乐' }
    ];
  }
  await refresh();
});
</script>

<template>
  <div>
    <header>
      <h1>🎵 LNH-MusicTag</h1>
      <span class="badge">Go + go-taglib(WASM) + 多源刮削</span>
      <span class="badge">{{ state.fpActive ? '指纹: 已启用' : '指纹: 未启用' }}</span>
      <span class="navbar"></span>
      <button class="navlink" @click="go('/settings')">⚙️ 设置</button>
      <button class="ghost sm" @click="logout">退出</button>
    </header>
    <div class="wrap">
      <div class="cards">
        <div class="card"><div class="num">{{ state.tracks.length }}</div><div class="lab">曲目</div></div>
        <div class="card"><div class="num">{{ state.dupGroups.length }}</div><div class="lab">重复分组</div></div>
        <div class="card"><div class="num">{{ totalMin }}</div><div class="lab">总时长(分钟)</div></div>
      </div>

      <div class="panel">
        <h2>① 扫描目录</h2>
        <div class="row">
          <input type="text" v-model="state.scanDir" placeholder="输入容器内音频目录，如 /music" @keydown.enter="scan" />
          <button @click="scan">扫描</button>
        </div>
        <div class="loading">{{ state.scanMsg }}</div>
      </div>

      <div class="grid2">
        <div class="panel">
          <h2>② 曲目列表</h2>
          <div style="max-height:520px;overflow:auto"><TrackTable /></div>
        </div>
        <div>
          <div class="panel"><h2>③ 编辑 / 刮削</h2><EditPanel /></div>
          <div class="panel"><h2>④ 重复检测</h2><DedupPanel /></div>
          <div class="panel"><h2>⑤ 批量刮削</h2><BatchPanel /></div>
        </div>
      </div>
    </div>
  </div>
</template>
