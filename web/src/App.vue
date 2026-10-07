<script setup>
import { onMounted } from 'vue'
import { state } from './store.js'
import { api } from './api.js'
import Login from './views/Login.vue'
import Main from './views/Main.vue'
import Settings from './views/Settings.vue'
import Toast from './components/Toast.vue'

window.addEventListener('hashchange', () => {
  state.route = location.hash.slice(1) || '/';
});
window.addEventListener('unauth', () => {
  state.authed = false;
  state.route = '/';
  location.hash = '/';
});

onMounted(async () => {
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
  <Settings v-else-if="state.route === '/settings'" />
  <Main v-else />
  <Toast />
</template>
