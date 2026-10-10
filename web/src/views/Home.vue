<script setup>
import { ref } from 'vue'
import { state } from '../store.js'
import { t } from '../i18n.js'
import Login from './Login.vue'

// 公开首页：仅展示项目介绍；搜索区默认隐藏（后续接入功能时启用）。
const q = ref('')
const showLogin = ref(false)

function doSearch() {
  // 搜索功能暂未接入，保留入口供后续开发。
  if (!q.value.trim()) return;
}

function enterApp() {
  if (state.authed) { state.view = 'app'; }
  else { showLogin.value = true; }
}
</script>

<template>
  <div class="home">
    <div class="home-hero">
      <div class="home-logo">🎵</div>
      <h1 class="home-title">LNH-MusicTag</h1>
      <p class="home-tag">{{ t('homeTagline') }}</p>

      <!-- 搜索区（默认隐藏，接入功能时移除 style="display:none" 即可启用） -->
      <div class="home-search" style="display:none">
        <input type="text" v-model="q" :placeholder="t('homeSearchPh')" @keydown.enter="doSearch" />
        <button @click="doSearch">{{ t('homeSearchBtn') }}</button>
      </div>

      <div class="home-actions">
        <button class="primary" @click="enterApp">{{ state.authed ? t('enterAdmin') : t('loginAdmin') }}</button>
        <button v-if="state.authed" class="ghost" @click="state.view = 'home'">{{ t('home') }}</button>
      </div>
    </div>

    <div class="home-feats">
      <div class="feat" v-for="f in ['featTags', 'featScrape', 'featDedup', 'featBatch', 'featConvert', 'featIdentify']" :key="f">
        {{ t(f) }}
      </div>
    </div>

    <p class="home-foot muted">{{ t('homeFoot') }}</p>

    <!-- 登录（后台）弹层 -->
    <Login v-if="showLogin" @done="showLogin = false" />
  </div>
</template>

<style scoped>
.home { min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 28px; padding: 24px; }
.home-hero { text-align: center; display: flex; flex-direction: column; align-items: center; gap: 14px; }
.home-logo { font-size: 56px; line-height: 1; }
.home-title { margin: 0; font-size: 34px; letter-spacing: 1px; }
.home-tag { margin: 0; color: var(--muted, rgba(128,128,128,.8)); }
.home-search { display: flex; gap: 8px; width: min(520px, 92vw); }
.home-search input { flex: 1; padding: 11px 14px; border-radius: 10px; border: 1px solid rgba(128,128,128,.3); background: var(--bg2, rgba(128,128,128,.08)); color: inherit; font-size: 15px; }
.home-search button { padding: 0 22px; border-radius: 10px; border: 0; background: var(--accent, #2f80ed); color: #fff; font-weight: 600; cursor: pointer; }
.home-actions { display: flex; gap: 10px; margin-top: 6px; }
.home-actions .primary { padding: 9px 24px; border-radius: 10px; border: 0; background: var(--accent, #2f80ed); color: #fff; font-weight: 600; cursor: pointer; }
.home-actions .ghost { padding: 9px 20px; border-radius: 10px; border: 1px solid rgba(128,128,128,.35); background: transparent; color: inherit; cursor: pointer; }
.home-feats { display: flex; flex-wrap: wrap; justify-content: center; gap: 10px; max-width: 720px; }
.feat { padding: 8px 16px; border-radius: 20px; background: var(--bg2, rgba(128,128,128,.08)); font-size: 13px; }
.home-foot { font-size: 12px; }
</style>
