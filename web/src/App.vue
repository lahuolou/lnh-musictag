<script setup>
import { onMounted, ref } from 'vue'
import { state, logout } from './store.js'
import { api } from './api.js'
import { t, lang, setLang } from './i18n.js'
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
function toggleLang() { setLang(lang.value === 'zh' ? 'en' : 'zh'); }

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
      <span class="brand">{{ t('brand') }}</span>
      <span class="spacer"></span>
      <span class="badge">{{ t('tracks', { n: state.tracks.length }) }}</span>
      <!-- 全局批量进度：后台任务进行中，任意页面可见，点击回到批量页 -->
      <span v-if="state.batchJob && state.batchJob.status !== 'done'" class="badge batch" :title="t('batchRunningTitle')" @click="tab('batch')">
        {{ t('batchProgress', { d: state.batchJob.done, t: state.batchJob.total }) }}
      </span>
      <button class="ghost sm" :title="lang === 'zh' ? 'Switch to English' : '切换到中文'" @click="toggleLang">{{ lang === 'zh' ? 'EN' : '中' }}</button>
      <button class="navlink" :title="theme === 'dark' ? t('lightMode') : t('darkMode')" @click="toggleTheme">{{ theme === 'dark' ? '🌙' : '☀️' }}</button>
      <button class="ghost sm" :class="{ on: state.activeTab !== 'settings' }" @click="tab('list')">{{ t('list') }}</button>
      <button class="ghost sm" :class="{ on: state.activeTab === 'settings' }" @click="tab('settings')">{{ t('settings') }}</button>
      <button class="ghost sm" @click="logout">{{ t('logout') }}</button>
    </nav>

    <!-- 合并页：左 音乐列表，右 编辑/批量/去重 -->
    <div v-if="state.activeTab !== 'settings'" class="page merge">
      <div class="split">
        <div class="left">
          <MusicTree />
        </div>
        <div class="right">
          <div class="subtabs">
            <button class="tab" :class="{ on: state.activeTab === 'edit' }" @click="tab('edit')">{{ t('edit') }}</button>
            <button class="tab" :class="{ on: state.activeTab === 'batch' }" @click="tab('batch')">{{ t('batch') }}</button>
            <button class="tab" :class="{ on: state.activeTab === 'dedup' }" @click="tab('dedup')">{{ t('dedup') }}</button>
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
