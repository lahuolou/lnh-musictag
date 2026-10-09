<script setup>
import { computed, onMounted, ref } from 'vue'
import { toast } from '../store.js'
import { api } from '../api.js'
import { t } from '../i18n.js'

// 插件页：按类型分组渲染全部插件（刮削源 / 工具 / 下载·协议预留），
// 开关即启停并持久化；FFmpeg 支持自定义路径 / 探测 / 容器内一键安装。
const plugins = ref([])
const busy = ref(false)
const msg = ref('')
const ffStatus = ref({ available: false, path: '', version: '' })
const ffInstalling = ref(false)

const groupMeta = [
  { kind: 'scrape', key: 'srcPlugins', hint: 'srcPluginsHint' },
  { kind: 'tool', key: 'ffPlugins', hint: 'ffPluginsHint' },
  { kind: 'download', key: 'dlPlugins', hint: 'dlPluginsHint' },
  { kind: 'protocol', key: 'protoPlugins', hint: 'protoPluginsHint' },
]

// 需 Key 源 -> 设置页字段文案的 i18n 键（提示用户去「设置 → 数据库配置」填写）
const keyLabelMap = {
  lastfm_key: 'fLastfm',
  spotify_id: 'fSpotifyId',
  discogs_token: 'fDiscogs',
  jamendo_client_id: 'fJamendo',
}
function keyLabel(p) { return keyLabelMap[p.keyField] || '' }

const groups = computed(() => {
  const g = { scrape: [], tool: [], download: [], protocol: [] }
  for (const p of plugins.value) if (g[p.kind]) g[p.kind].push(p)
  return g
})

function ffPlugin() { return plugins.value.find(p => p.name === 'tool.ffmpeg') }

async function loadPlugins() {
  try { plugins.value = (await api('/api/plugins') || {}).plugins || []; }
  catch (e) { msg.value = t('cfgLoadFail', { m: e.message }); }
}
async function loadFFmpeg() {
  try { ffStatus.value = await api('/api/ffmpeg/status'); }
  catch (e) { /* 静默 */ }
}

async function togglePlugin(p) {
  busy.value = true;
  try {
    await api('/api/plugins', { method: 'POST', body: JSON.stringify({ [p.name]: { enabled: !p.enabled } }) });
    p.enabled = !p.enabled;
    toast(t('srcSaved'), 'ok');
  } catch (e) { toast(t('srcSaveFail', { m: e.message }), 'err'); }
  finally { busy.value = false; }
}

async function savePluginConfig(p) {
  busy.value = true;
  msg.value = '';
  try {
    await api('/api/plugins', { method: 'POST', body: JSON.stringify({ [p.name]: { config: { ...p.config } } }) });
    toast(t('cfgSaved'), 'ok');
    if (p.name === 'tool.ffmpeg') { loadFFmpeg(); }
  } catch (e) { msg.value = t('cfgSaveFail', { m: e.message }); }
  finally { busy.value = false; }
}

async function probeFFmpeg() {
  msg.value = '';
  const fp = ffPlugin();
  if (fp && fp.config && fp.config.path) await savePluginConfig(fp);
  await loadFFmpeg();
  toast(ffStatus.value.available ? t('ffReady') + ' · ' + ffStatus.value.version : t('ffMissing'), ffStatus.value.available ? 'ok' : 'err');
}

async function installFFmpeg() {
  ffInstalling.value = true;
  msg.value = '';
  try {
    const r = await api('/api/ffmpeg/install', { method: 'POST', body: '{}' });
    toast(t('ffInstallOk'), 'ok');
    if (r.path) { const fp = ffPlugin(); if (fp) { fp.config = fp.config || {}; fp.config.path = r.path; } }
    await loadFFmpeg();
  } catch (e) {
    toast(t('ffInstallFail', { m: e.message }), 'err');
    msg.value = e.message;
  } finally { ffInstalling.value = false; }
}

onMounted(() => { loadPlugins(); loadFFmpeg(); });
</script>

<template>
  <div class="page-wrap">
    <h1 class="pg-title">{{ t('plugins') }}</h1>

    <div class="panel" v-for="gm in groupMeta" :key="gm.kind" v-show="groups[gm.kind].length">
      <h2>{{ t(gm.key) }}</h2>
      <p class="muted" style="margin:0 0 12px">{{ t(gm.hint) }}</p>

      <div class="plist">
        <div class="pitem" v-for="p in groups[gm.kind]" :key="p.name">
          <div class="prow">
            <span class="plabel">
              {{ p.label }}
              <small class="muted">{{ p.name }}</small>
              <em v-if="p.builtin" class="builtin">{{ t('builtin') }}</em>
              <em v-if="p.needsKey" class="key" :class="{ ok: p.keyConfigured }">🔑 {{ p.keyConfigured ? t('keySet') : t('keyNotSet') }}</em>
            </span>
            <label class="switch" :class="{ on: p.enabled }">
              <input type="checkbox" :checked="p.enabled" :disabled="busy || p.builtin" @change="togglePlugin(p)" />
              <span class="slider"></span>
            </label>
          </div>

          <!-- 需 Key 源：未配置时引导去设置页填写 -->
          <div v-if="p.needsKey && !p.keyConfigured" class="key-tip">
            → {{ t('keyGoSetting') }}「{{ t(keyLabel(p)) }}」
          </div>
          <div v-else-if="p.needsKey && !p.enabled" class="key-tip dim">
            {{ t('keyReadyToEnable') }}
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
              <button class="sm" :disabled="busy" @click="savePluginConfig(p)">{{ t('saveCfg') }}</button>
              <button v-if="p.installable" class="sm secondary" :disabled="ffInstalling" @click="installFFmpeg">{{ ffInstalling ? t('ffInstalling') : t('ffInstall') }}</button>
              <button v-if="p.name === 'tool.ffmpeg'" class="sm secondary" @click="probeFFmpeg">{{ t('ffProbe') }}</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="panel" v-if="!msg && !plugins.length" style="padding:24px;text-align:center">
      <p class="muted">{{ t('pluginsEmpty') }}</p>
    </div>

    <div class="panel" v-if="msg">
      <p class="muted">{{ msg }}</p>
    </div>
  </div>
</template>

<style scoped>
.pg-title { margin: 0 0 14px; font-size: 20px; }
.plist { display: flex; flex-direction: column; gap: 8px; }
.pitem { padding: 10px; border-radius: 8px; background: var(--bg2, rgba(128,128,128,.08)); }
.prow { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.plabel { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.plabel small { opacity: .6; }
.builtin, .key { font-style: normal; font-size: 11px; padding: 1px 6px; border-radius: 6px; background: rgba(128,128,128,.18); }
.key { background: rgba(230,148,61,.22); color: #e6943d; }
.key.ok { background: rgba(48,164,108,.2); color: #30a46c; }
.key-tip { margin-top: 6px; font-size: 12px; color: #e6943d; }
.key-tip.dim { color: rgba(128,128,128,.7); }
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
