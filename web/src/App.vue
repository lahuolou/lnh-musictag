<script setup>
import { onMounted, ref, watch } from 'vue'
import { state, logout, resumeJobs, verState, checkVersion } from './store.js'
import { api } from './api.js'
import { t, lang, setLang } from './i18n.js'
import Home from './views/Home.vue'
import Login from './views/Login.vue'
import MusicTree from './components/MusicTree.vue'
import EditPanel from './components/EditPanel.vue'
import BatchPanel from './components/BatchPanel.vue'
import DedupPanel from './components/DedupPanel.vue'
import Settings from './views/Settings.vue'
import Plugins from './views/Plugin.vue'
import Toast from './components/Toast.vue'
import UpdateModal from './components/UpdateModal.vue'
import AboutModal from './components/AboutModal.vue'

const theme = ref(localStorage.getItem('lnh-theme') || (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'))
function applyTheme() {
  document.documentElement.dataset.theme = theme.value;
  localStorage.setItem('lnh-theme', theme.value);
}
function toggleTheme() { theme.value = theme.value === 'dark' ? 'light' : 'dark'; applyTheme(); }

function tab(name) { state.activeTab = name; }
function toggleLang() { setLang(lang.value === 'zh' ? 'en' : 'zh'); }
function goHome() { state.view = 'home'; }

const aboutOpen = ref(false)
const gearOpen = ref(false)

// 登录成功后自动从公开首页进入后台
watch(() => state.authed, v => { if (v) state.view = 'app'; })

onMounted(async () => {
  applyTheme();
  try {
    const m = await api('/api/me');
    state.authed = true;
    state.user = m.user;
    state.view = 'app'; // 已有会话（刷新恢复）直接进后台
  } catch (e) {
    state.authed = false;
  }
  checkVersion(); // 已登录（如刷新恢复会话）时检查更新；未登录时静默失败，登录成功后由 Login.vue 再查
  if (state.authed) resumeJobs(); // 刷新/换设备后恢复进行中的批量任务进度
});
</script>

<template>
  <!-- 公开首页：搜索占位，无需登录；登录/进入后台切到后台区 -->
  <Home v-if="state.view === 'home'" />

  <!-- 后台区：登录页 + 登录后的全部管理页面 -->
  <template v-else>
    <Login v-if="!state.authed" />
    <template v-else>
    <nav class="topnav">
      <span class="brand">{{ t('brand') }}</span>
      <span class="spacer"></span>
      <span class="badge">{{ t('tracks', { n: state.tracks.length }) }}</span>
      <!-- 新版本徽标：点击打开更新弹窗 -->
      <a v-if="verState.ver" class="badge newver" :title="t('newVerTitle')" @click="verState.showUpdate = true">{{ t('newVer') }} v{{ verState.ver.latest }}</a>
      <!-- 全局批量进度：后台任务进行中，任意页面可见，点击回到批量页 -->
      <span v-if="state.batchJob && state.batchJob.status !== 'done'" class="badge batch" :title="t('batchRunningTitle')" @click="tab('batch')">
        {{ t('batchProgress', { d: state.batchJob.done, t: state.batchJob.total }) }}
      </span>
      <!-- 通用工具任务进度（识别/转换/乱码/简繁）：后台运行，任意页面可见 -->
      <span v-if="state.toolJob && state.toolJob.status === 'running'" class="badge batch" :title="t('jobRunningTitle')" @click="tab('batch')">
        {{ t('jobRunning', { d: state.toolJob.done, t: state.toolJob.total, c: '' }) }}
      </span>
      <button class="ghost sm" :title="lang === 'zh' ? 'Switch to English' : '切换到中文'" @click="toggleLang">{{ lang === 'zh' ? 'EN' : '中' }}</button>
      <button class="navlink" :title="theme === 'dark' ? t('lightMode') : t('darkMode')" @click="toggleTheme">{{ theme === 'dark' ? '🌙' : '☀️' }}</button>
      <button class="ghost sm" :class="{ on: state.activeTab !== 'settings' && state.activeTab !== 'plugins' }" @click="tab('list')">{{ t('list') }}</button>
      <button class="ghost sm" :title="t('home')" @click="goHome">🏠</button>
      <!-- 齿轮二级导航：设置 / 关于 / 检查更新 / 退出（悬停或点击弹出） -->
      <div class="gear" :class="{ open: gearOpen }" @click="gearOpen = !gearOpen">
        <button class="ghost sm gear-btn" :title="t('menu')">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        </button>
        <div class="gear-menu" @click.stop>
          <a @click="tab('settings'); gearOpen = false">{{ t('settings') }}</a>
          <a @click="tab('plugins'); gearOpen = false">{{ t('plugins') }}</a>
          <a @click="aboutOpen = true; gearOpen = false">{{ t('about') }}</a>
          <a @click="checkVersion(true); gearOpen = false">{{ t('checkUpdate') }}</a>
          <a class="danger" @click="logout">{{ t('logout') }}</a>
        </div>
      </div>
    </nav>

    <!-- 合并页：左 音乐列表，右 编辑/批量/去重 -->
    <div v-if="state.activeTab !== 'settings' && state.activeTab !== 'plugins'" class="page merge">
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

    <div v-else-if="state.activeTab === 'plugins'" class="page">
      <Plugins />
    </div>

    <div v-else class="page">
      <Settings />
    </div>
    </template>
  </template>
  <UpdateModal :open="verState.showUpdate" :data="verState.ver" @close="verState.showUpdate = false" @later="verState.showUpdate = false" @view="verState.showUpdate = false" />
  <AboutModal :open="aboutOpen" @close="aboutOpen = false" />
  <Toast />
</template>
