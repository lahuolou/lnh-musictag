<script setup>
import { ref } from 'vue'
import { state } from '../store.js'
import { api } from '../api.js'

const user = ref('admin')
const pass = ref('')
const msg = ref('')
const showPw = ref(false)

async function doLogin() {
  msg.value = '';
  if (!user.value.trim() || !pass.value) { msg.value = '请输入账号和密码'; return; }
  msg.value = '登录中…';
  try {
    await api('/api/login', { method: 'POST', body: JSON.stringify({ user: user.value.trim(), pass: pass.value }) });
    state.authed = true;
    state.user = user.value.trim();
  } catch (e) { msg.value = e.message; }
}
</script>

<template>
  <div class="login-overlay">
    <div class="login-box">
      <div class="login-brand">🎵</div>
      <h2>LNH-MusicTag</h2>
      <p class="login-sub">登录后台管理音乐库</p>
      <div class="field">
        <label>账号</label>
        <input type="text" v-model="user" autocomplete="username" @keydown.enter="doLogin" />
      </div>
      <div class="field">
        <label>密码</label>
        <div class="pw-row">
          <input :type="showPw ? 'text' : 'password'" v-model="pass" autocomplete="current-password" @keydown.enter="doLogin" />
          <button class="ghost sm" type="button" @click="showPw = !showPw" title="显示/隐藏">👁</button>
        </div>
      </div>
      <button class="login-btn" @click="doLogin">登录</button>
      <div class="loading login-msg">{{ msg }}</div>
    </div>
  </div>
</template>
