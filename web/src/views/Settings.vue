<script setup>
import { onMounted, ref } from 'vue'
import { state, toast } from '../store.js'
import { api } from '../api.js'
import { t } from '../i18n.js'

const oldPass = ref('')
const newPass = ref('')
const confirm = ref('')
const msg = ref('')

const cfg = ref({ adminUser: '', acoustidKey: '', lastfmKey: '', discogsToken: '', jamendoClientID: '', spotifyID: '', spotifySecret: '', autoFixTitle: true, autoRenameFile: true, scanDir: '' })
const cfgMsg = ref('')

async function doChange() {
  msg.value = '';
  if (newPass.value !== confirm.value) { msg.value = t('passMismatch'); return; }
  if (newPass.value.length < 6) { msg.value = t('passTooShort'); return; }
  try {
    await api('/api/change-password', { method: 'POST', body: JSON.stringify({ oldPass: oldPass.value, newPass: newPass.value }) });
    toast(t('passChanged'), 'ok');
    oldPass.value = newPass.value = confirm.value = '';
    msg.value = '';
  } catch (e) { msg.value = e.message; }
}

async function loadCfg() {
  try { cfg.value = await api('/api/settings'); }
  catch (e) { toast(t('cfgLoadFail', { m: e.message }), 'err'); }
}

async function saveCfg() {
  cfgMsg.value = '';
  try {
    await api('/api/settings', {
      method: 'POST',
      body: JSON.stringify({
        adminUser: cfg.value.adminUser,
        acoustidKey: cfg.value.acoustidKey,
        lastfmKey: cfg.value.lastfmKey,
        discogsToken: cfg.value.discogsToken,
        jamendoClientID: cfg.value.jamendoClientID,
        spotifyID: cfg.value.spotifyID,
        spotifySecret: cfg.value.spotifySecret,
        autoFixTitle: cfg.value.autoFixTitle,
        autoRenameFile: cfg.value.autoRenameFile,
        scanDir: cfg.value.scanDir
      })
    });
    toast(t('cfgSaved'), 'ok');
    cfgMsg.value = t('cfgSavedDb');
  } catch (e) { cfgMsg.value = t('cfgSaveFail', { m: e.message }); }
}

onMounted(loadCfg);
</script>

<template>
  <div class="grid2" style="align-items:start">
    <div>
      <div class="panel">
        <h2>{{ t('changePass') }}</h2>
        <p class="muted" style="margin:0 0 10px">{{ t('changePassHint') }}</p>
        <div class="field"><label>{{ t('oldPass') }}</label><input type="password" v-model="oldPass" autocomplete="current-password" /></div>
        <div class="field"><label>{{ t('newPass') }}</label><input type="password" v-model="newPass" autocomplete="new-password" /></div>
        <div class="field"><label>{{ t('confirmPass') }}</label><input type="password" v-model="confirm" autocomplete="new-password" @keydown.enter="doChange" /></div>
        <div class="row">
          <button @click="doChange">{{ t('confirmBtn') }}</button>
        </div>
        <div class="loading login-msg">{{ msg }}</div>
      </div>

      <div class="panel">
        <h2>{{ t('tip') }}</h2>
        <p class="muted">{{ t('tipText') }}</p>
      </div>
    </div>

    <div class="panel">
      <h2>{{ t('dbCfg') }}</h2>
      <div class="field">
        <label>{{ t('fAdmin') }}</label>
        <input type="text" v-model="cfg.adminUser" />
      </div>
      <div class="field">
        <label>{{ t('fAcoustid') }}</label>
        <input type="text" v-model="cfg.acoustidKey" :placeholder="t('optional')" />
      </div>
      <div class="field">
        <label>{{ t('fLastfm') }}</label>
        <input type="text" v-model="cfg.lastfmKey" placeholder="https://www.last.fm/api/account/create" />
      </div>
      <div class="field">
        <label>{{ t('fDiscogs') }}</label>
        <input type="text" v-model="cfg.discogsToken" placeholder="https://www.discogs.com/settings/developers" />
      </div>
      <div class="field">
        <label>{{ t('fJamendo') }}</label>
        <input type="text" v-model="cfg.jamendoClientID" placeholder="https://devportal.jamendo.com" />
      </div>
      <div class="field">
        <label>{{ t('fSpotifyId') }}</label>
        <input type="text" v-model="cfg.spotifyID" placeholder="https://developer.spotify.com/dashboard" />
      </div>
      <div class="field">
        <label>{{ t('fSpotifySecret') }}</label>
        <input type="password" v-model="cfg.spotifySecret" :placeholder="t('fSpotifySecret')" />
      </div>
      <div class="field">
        <label>{{ t('fScanDir') }}</label>
        <input type="text" v-model="cfg.scanDir" placeholder="/music" />
      </div>
      <div class="chk" style="margin:6px 0 14px">
        <input type="checkbox" id="autoFix" class="tchk" v-model="cfg.autoFixTitle" />
        <label for="autoFix">{{ t('optAutoFix') }}</label>
      </div>
      <div class="chk" style="margin:0 0 14px">
        <input type="checkbox" id="autoRename" class="tchk" v-model="cfg.autoRenameFile" />
        <label for="autoRename">{{ t('optAutoRename') }}</label>
      </div>
      <div class="row">
        <button @click="saveCfg">{{ t('saveCfg') }}</button>
      </div>
      <div class="loading login-msg">{{ cfgMsg }}</div>
    </div>
  </div>
</template>
