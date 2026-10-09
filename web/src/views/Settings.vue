<script setup>
import { computed, onMounted, ref } from 'vue'
import { toast } from '../store.js'
import { api } from '../api.js'
import { t } from '../i18n.js'

const oldPass = ref('')
const newPass = ref('')
const confirm = ref('')
const msg = ref('')

const cfg = ref({ adminUser: '', acoustidKey: '', lastfmKey: '', discogsToken: '', jamendoClientID: '', spotifyID: '', spotifySecret: '', autoFixTitle: true, autoRenameFile: true, scanDir: '' })
const cfgMsg = ref('')

// 通用插件（刮削源 / 工具 / 预留类型）
const plugins = ref([])
const plBusy = ref(false)
const plMsg = ref('')

// FFmpeg 实时状态
const ffStatus = ref({ available: false, path: '', version: '' })
const ffInstalling = ref(false)

const groupMeta = {
  scrape: { key: 'srcPlugins', hint: 'srcPluginsHint', icon: '🔌' },
  tool: { key: 'ffPlugins', hint: 'ffHint', icon: '🎛️' },
  download: { key: 'dlPlugins', hint: 'dlPluginsHint', icon: '⬇️' },
  protocol: { key: 'protoPlugins', hint: 'protoPluginsHint', icon: '🌐' },
}

const groups = computed(() => {
  const g = { scrape: [], tool: [], download: [], protocol: [] }
  for (const p of plugins.value) if (g[p.kind]) g[p.kind].push(p)
  return g
})

function ffPlugin() { return plugins.value.find(p => p.name === 'tool.ffmpeg') }

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

async function loadPlugins() {
  try { plugins.value = (await api('/api/plugins') || {}).plugins || []; }
  catch (e) { plMsg.value = t('cfgLoadFail', { m: e.message }); }
}

async function loadFFmpeg() {
  try { ffStatus.value = await api('/api/ffmpeg/status'); }
  catch (e) { /* 静默 */ }
}

// 开关插件（内置 auto 不可停用）
async function togglePlugin(p) {
  plBusy.value = true;
  try {
    await api('/api/plugins', { method: 'POST', body: JSON.stringify({ [p.name]: { enabled: !p.enabled } }) });
    p.enabled = !p.enabled;
    toast(t('srcSaved'), 'ok');
  } catch (e) { toast(t('srcSaveFail', { m: e.message }), 'err'); }
  finally { plBusy.value = false; }
}

// 保存某插件的配置字段
async function savePluginConfig(p) {
  plBusy.value = true;
  plMsg.value = '';
  try {
    await api('/api/plugins', { method: 'POST', body: JSON.stringify({ [p.name]: { config: { ...p.config } } }) });
    toast(t('cfgSaved'), 'ok');
    if (p.name === 'tool.ffmpeg') { loadFFmpeg(); }
  } catch (e) { plMsg.value = t('cfgSaveFail', { m: e.message }); }
  finally { plBusy.value = false; }
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

async function probeFFmpeg() {
  plMsg.value = '';
  const fp = ffPlugin();
  if (fp && fp.config && fp.config.path) await savePluginConfig(fp);
  await loadFFmpeg();
  toast(ffStatus.value.available ? t('ffReady') + ' · ' + ffStatus.value.version : t('ffMissing'), ffStatus.value.available ? 'ok' : 'err');
}

async function installFFmpeg() {
  ffInstalling.value = true;
  plMsg.value = '';
  try {
    const r = await api('/api/ffmpeg/install', { method: 'POST', body: '{}' });
    toast(t('ffInstallOk'), 'ok');
    if (r.path) { const fp = ffPlugin(); if (fp) { fp.config = fp.config || {}; fp.config.path = r.path; } }
    await loadFFmpeg();
  } catch (e) {
    toast(t('ffInstallFail', { m: e.message }), 'err');
    plMsg.value = e.message;
  } finally { ffInstalling.value = false; }
}

onMounted(() => { loadCfg(); loadPlugins(); loadFFmpeg(); });
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

    <div>
      <div class="panel">
        <h2>{{ t('dbCfg') }}</h2>
        <div class="field"><label>{{ t('fAdmin') }}</label><input type="text" v-model="cfg.adminUser" /></div>
        <div class="field"><label>{{ t('fAcoustid') }}</label><input type="text" v-model="cfg.acoustidKey" :placeholder="t('optional')" /></div>
        <div class="field"><label>{{ t('fLastfm') }}</label><input type="text" v-model="cfg.lastfmKey" placeholder="https://www.last.fm/api/account/create" /></div>
        <div class="field"><label>{{ t('fDiscogs') }}</label><input type="text" v-model="cfg.discogsToken" placeholder="https://www.discogs.com/settings/developers" /></div>
        <div class="field"><label>{{ t('fJamendo') }}</label><input type="text" v-model="cfg.jamendoClientID" placeholder="https://devportal.jamendo.com" /></div>
        <div class="field"><label>{{ t('fSpotifyId') }}</label><input type="text" v-model="cfg.spotifyID" placeholder="https://developer.spotify.com/dashboard" /></div>
        <div class="field"><label>{{ t('fSpotifySecret') }}</label><input type="password" v-model="cfg.spotifySecret" :placeholder="t('fSpotifySecret')" /></div>
        <div class="field"><label>{{ t('fScanDir') }}</label><input type="text" v-model="cfg.scanDir" placeholder="/music" /></div>
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

      <!-- 插件分组：刮削源 / 工具 / 下载（预留）/ 协议（预留） -->
      <div class="panel" v-for="(kind, key) in groupMeta" :key="key">
        <template v-if="groups[key].length">
          <h2>{{ t(kind.key) }}</h2>
          <p class="muted" style="margin:0 0 10px">{{ t(kind.hint) }}</p>

          <div class="plist">
            <div class="pitem" v-for="p in groups[key]" :key="p.name">
              <div class="prow">
                <span class="plabel">{{ p.label }} <small class="muted">{{ p.name }}</small></span>
                <label class="switch" :class="{ on: p.enabled }">
                  <input type="checkbox" :checked="p.enabled" :disabled="plBusy || p.builtin" @change="togglePlugin(p)" />
                  <span class="slider"></span>
                </label>
              </div>

              <!-- FFmpeg 实时状态 -->
              <div v-if="p.name === 'tool.ffmpeg'" class="ff-status" :class="{ ok: ffStatus.available }" style="margin:8px 0 2px">
                <span class="dot"></span>
                {{ ffStatus.available ? t('ffReady') : t('ffMissing') }}
                <span v-if="ffStatus.available" class="muted"> · {{ t('ffVersion') }}: {{ ffStatus.version }}</span>
              </div>

              <!-- 配置字段表单 -->
              <div v-if="p.fields && p.fields.length" class="pfields">
                <div class="field" v-for="f in p.fields" :key="f.key" style="margin-top:6px">
                  <label>{{ t(f.label) }}</label>
                  <input :type="f.type === 'password' ? 'password' : 'text'" v-model="p.config[f.key]" :placeholder="t(f.placeholder || '')" />
                </div>
                <div class="row" style="margin-top:8px">
                  <button class="sm" :disabled="plBusy" @click="savePluginConfig(p)">{{ t('saveCfg') }}</button>
                  <button v-if="p.installable" class="sm secondary" :disabled="ffInstalling" @click="installFFmpeg">{{ ffInstalling ? t('ffInstalling') : t('ffInstall') }}</button>
                  <button v-if="p.name === 'tool.ffmpeg'" class="sm secondary" @click="probeFFmpeg">{{ t('ffProbe') }}</button>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>

      <div class="panel" v-if="plMsg">
        <p class="muted">{{ plMsg }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.plist { display: flex; flex-direction: column; gap: 8px; }
.pitem { padding: 10px; border-radius: 8px; background: var(--bg2, rgba(128,128,128,.08)); }
.prow { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.plabel small { margin-left: 6px; opacity: .6; }
.switch { position: relative; display: inline-block; width: 40px; height: 22px; flex: none; }
.switch input { opacity: 0; width: 0; height: 0; }
.slider { position: absolute; cursor: pointer; inset: 0; background: rgba(128,128,128,.35); border-radius: 22px; transition: .2s; }
.slider::before { content: ''; position: absolute; width: 16px; height: 16px; left: 3px; top: 3px; background: #fff; border-radius: 50%; transition: .2s; }
.switch.on .slider { background: var(--accent, #2f80ed); }
.switch.on .slider::before { transform: translateX(18px); }
.ff-status { display: flex; align-items: center; gap: 6px; font-weight: 600; }
.ff-status .dot { width: 10px; height: 10px; border-radius: 50%; background: #e5484d; }
.ff-status.ok .dot { background: #30a46c; }
</style>
