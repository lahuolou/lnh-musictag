<script setup>
import { onMounted, ref } from 'vue'
import { state, logout } from './store.js'
import { api } from './api.js'
import Login from './views/Login.vue'
import Tracks from './views/Tracks.vue'
import Edit from './views/Edit.vue'
import Dedup from './views/Dedup.vue'
import Batch from './views/Batch.vue'
import Settings from './views/Settings.vue'
import Toast from './components/Toast.vue'

const theme = ref(localStorage.getItem('lnh-theme') || (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'))
function applyTheme() {
  document.documentElement.dataset.theme = theme.value;
  localStorage.setItem('lnh-theme', theme.value);
}
function toggleTheme() { theme.value = theme.value === 'dark' ? 'light' : 'dark'; applyTheme(); }

function route() {
  const h = state.route || location.hash.slice(1) || '/';
  if (h.startsWith('/edit')) return 'edit';
  if (h === '/dedup') return 'dedup';
  if (h === '/batch') return 'batch';
  if (h === '/settings') return 'settings';
  return 'tracks';
}
function go(h) { state.route = h; location.hash = h; }

window.addEventListener('hashchange', () => {
  state.route = location.hash.slice(1) || '/';
});
window.addEventListener('unauth', () => {
  state.authed = false;
  state.route = '/';
  location.hash = '/';
});

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
      <button class="navlink" :class="{ on: route() === 'tracks' }" @click="go('/')">曲目</button>
      <button class="navlink" :class="{ on: route() === 'edit' }" @click="go('/edit/')">编辑</button>
      <button class="navlink" :class="{ on: route() === 'dedup' }" @click="go('/dedup')">去重</button>
      <button class="navlink" :class="{ on: route() === 'batch' }" @click="go('/batch')">批量</button>
      <span class="spacer"></span>
      <span class="badge">{{ state.fpActive ? '指纹: 启用' : '指纹: 未启用' }}</span>
      <button class="navlink" :title="theme === 'dark' ? '切换到白天模式' : '切换到黑夜模式'" @click="toggleTheme">{{ theme === 'dark' ? '🌙' : '☀️' }}</button>
      <button class="navlink" :class="{ on: route() === 'settings' }" @click="go('/settings')">⚙️ 设置</button>
      <button class="ghost sm" @click="logout">退出</button>
    </nav>
    <div class="wrap">
      <Tracks v-if="route() === 'tracks'" />
      <Edit v-else-if="route() === 'edit'" />
      <Dedup v-else-if="route() === 'dedup'" />
      <Batch v-else-if="route() === 'batch'" />
      <Settings v-else-if="route() === 'settings'" />
    </div>
  </template>
  <Toast />
</template>
