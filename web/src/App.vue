<script setup>
import { onMounted, ref } from 'vue'
import { state, logout } from './store.js'
import { api } from './api.js'
import Login from './views/Login.vue'
import MusicTree from './components/MusicTree.vue'
import EditPanel from './components/EditPanel.vue'
import BatchPanel from './components/BatchPanel.vue'
import DedupPanel from './components/DedupPanel.vue'
import Settings from './views/Settings.vue'
import Toast from './components/Toast.vue'

const theme = ref(localStorage.getItem('lnh-theme') || (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'))
function applyTheme() {
  document.documentElement.dataset.theme = theme.value;
  localStorage.setItem('lnh-theme', theme.value);
}
function toggleTheme() { theme.value = theme.value === 'dark' ? 'light' : 'dark'; applyTheme(); }

function tab(name) { state.activeTab = name; }

onMounted(async () => {
  applyTheme();
  try {
    const m = await api('/api/me');
    state.authed = true;
    state.user = m.user;
  } catch (e) {
    state.authed = false;
  }
});
</script>

<template>
  <Login v-if="!state.authed" />
  <template v-else>
    <nav class="topnav">
      <span class="brand">🎵 LNH-MusicTag</span>
      <span class="spacer"></span>
      <span class="badge">曲目 {{ state.tracks.length }}</span>
      <button class="navlink" :title="theme === 'dark' ? '切换到白天模式' : '切换到黑夜模式'" @click="toggleTheme">{{ theme === 'dark' ? '🌙' : '☀️' }}</button>
      <button class="ghost sm" :class="{ on: state.activeTab !== 'settings' }" @click="tab('list')">列表</button>
      <button class="ghost sm" :class="{ on: state.activeTab === 'settings' }" @click="tab('settings')">设置</button>
      <button class="ghost sm" @click="logout">退出</button>
    </nav>

    <!-- 合并页：左 音乐列表，右 编辑/批量/去重 -->
    <div v-if="state.activeTab !== 'settings'" class="page merge">
      <div class="split">
        <div class="left">
          <MusicTree />
        </div>
        <div class="right">
          <div class="subtabs">
            <button class="tab" :class="{ on: state.activeTab === 'edit' }" @click="tab('edit')">编辑</button>
            <button class="tab" :class="{ on: state.activeTab === 'batch' }" @click="tab('batch')">批量</button>
            <button class="tab" :class="{ on: state.activeTab === 'dedup' }" @click="tab('dedup')">去重</button>
          </div>
          <EditPanel v-if="state.activeTab === 'edit' || state.activeTab === 'list'" />
          <BatchPanel v-else-if="state.activeTab === 'batch'" />
          <DedupPanel v-else-if="state.activeTab === 'dedup'" />
        </div>
      </div>
    </div>

    <div v-else class="page">
      <Settings />
    </div>
  </template>
  <Toast />
</template>
