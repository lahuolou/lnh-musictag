<script setup>
import { ref } from 'vue'
import { state } from '../store.js'
import { api } from '../api.js'
import { t } from '../i18n.js'

const user = ref('admin')
const pass = ref('')
const msg = ref('')
const showPw = ref(false)
const busy = ref(false)

async function doLogin() {
  if (busy.value) return;
  msg.value = '';
  if (!user.value.trim() || !pass.value) { msg.value = t('loginEmpty'); return; }
  busy.value = true;
  msg.value = t('loggingIn');
  try {
    await api('/api/login', { method: 'POST', body: JSON.stringify({ user: user.value.trim(), pass: pass.value }) });
    state.authed = true;
    state.user = user.value.trim();
  } catch (e) { msg.value = e.message; }
  finally { busy.value = false; }
}
</script>

<template>
  <div class="login-overlay">
    <div class="login-box">
      <div class="login-brand">🎵</div>
      <h2>LNH-MusicTag</h2>
      <p class="login-sub">{{ t('loginSub') }}</p>
      <div class="field">
        <label>{{ t('username') }}</label>
        <input type="text" v-model="user" autocomplete="username" @keydown.enter="doLogin" />
      </div>
      <div class="field">
        <label>{{ t('password') }}</label>
        <div class="pw-row">
          <input :type="showPw ? 'text' : 'password'" v-model="pass" autocomplete="current-password" @keydown.enter="doLogin" />
          <button class="ghost sm" type="button" @click="showPw = !showPw" title="show/hide">👁</button>
        </div>
      </div>
      <button class="login-btn" :disabled="busy" @click="doLogin">{{ busy ? t('loggingIn') : t('loginBtn') }}</button>
      <div class="loading login-msg">{{ msg }}</div>
    </div>
  </div>
</template>
