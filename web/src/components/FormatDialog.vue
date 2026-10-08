<script setup>
import { computed, ref } from 'vue'
import { state, toast, refresh } from '../store.js'
import { api } from '../api.js'

const props = defineProps({ open: Boolean })
const emit = defineEmits(['close'])

const inputFormats = ref([]) // 空 = 全部格式
const target = ref('keep')
const bitrate = ref('keep')
const sampleRate = ref('keep')
const busy = ref(false)

const inFormats = ['FLAC', 'MP3', 'APE', 'WAV', 'AIFF', 'AIF', 'TTA', 'M4A', 'OGG', 'OPUS', 'WMA', 'DSF', 'DFF', 'WMV', 'DTS', 'AAC']
const outFormats = [
  { v: 'keep', label: '维持原格式' }, { v: 'mp3', label: 'MP3' }, { v: 'flac', label: 'FLAC' },
  { v: 'm4a', label: 'M4A' }, { v: 'ogg', label: 'OGG' }, { v: 'opus', label: 'OPUS' },
  { v: 'wav', label: 'WAV' }, { v: 'aiff', label: 'AIFF' }, { v: 'aac', label: 'AAC' },
  { v: 'wma', label: 'WMA' }, { v: 'tta', label: 'TTA' }
]
const bitrates = [
  { v: 'keep', label: '保持原样' }, { v: '64k', label: '64 kbps' }, { v: '128k', label: '128 kbps' },
  { v: '192k', label: '192 kbps' }, { v: '256k', label: '256 kbps' }, { v: '320k', label: '320 kbps' }
]
const rates = [
  { v: 'keep', label: '保持原样' }, { v: '44100', label: '44.1 kHz' }, { v: '48000', label: '48 kHz' },
  { v: '88200', label: '88.2 kHz' }, { v: '96000', label: '96 kHz' }
]

function toggleIn(f) {
  const i = inputFormats.value.indexOf(f);
  if (i >= 0) inputFormats.value.splice(i, 1); else inputFormats.value.push(f);
}

async function run() {
  if (!state.selected.size) { toast('请先在左侧列表勾选曲目', 'err'); return; }
  busy.value = true;
  try {
    const r = await api('/api/convert', {
      method: 'POST',
      body: JSON.stringify({
        ids: [...state.selected],
        target: target.value,
        inputFormats: inputFormats.value.map(x => x.toLowerCase()),
        bitrate: bitrate.value,
        sampleRate: sampleRate.value,
        removeSource: false
      })
    });
    const ok = (r.results || []).filter(x => x.ok).length;
    toast('格式转换完成：' + ok + '/' + r.total + ' 首', ok === r.total ? 'ok' : 'err');
    state.selected.clear();
    await refresh();
    emit('close');
  } catch (e) { toast('格式转换失败: ' + e.message, 'err'); }
  finally { busy.value = false; }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="modal-mask" @click.self="emit('close')">
      <div class="modal fmt">
        <div class="h3">格式转换</div>
        <div class="muted" style="margin-bottom:10px">批量转换音频格式、比特率和采样率。</div>
        <div class="muted" style="font-size:12px;line-height:1.7">
          将音频转换成其他格式，保存在同目录下<br />不会删除源文件，安全可靠
        </div>

        <div class="h4">输入设置</div>
        <div class="fl">输入格式</div>
        <div class="muted" style="font-size:12px">选择需要转换的音频格式</div>
        <div class="fmt-chips">
          <button v-for="f in inFormats" :key="f" class="chip" :class="{ on: inputFormats.includes(f) }" @click="toggleIn(f)">{{ f }}</button>
        </div>
        <div class="muted" style="font-size:12px">留空则对所有格式进行转换</div>

        <div class="h4">格式设置</div>
        <div class="fmt-grid">
          <div>
            <div class="fl">目标格式</div>
            <div class="muted" style="font-size:12px">选择音频输出格式</div>
            <select v-model="target" class="sel">
              <option v-for="o in outFormats" :key="o.v" :value="o.v">{{ o.label }}</option>
            </select>
          </div>
          <div>
            <div class="fl">比特率</div>
            <div class="muted" style="font-size:12px">音频质量设置</div>
            <select v-model="bitrate" class="sel">
              <option v-for="o in bitrates" :key="o.v" :value="o.v">{{ o.label }}</option>
            </select>
          </div>
        </div>
        <div style="margin-top:10px">
          <div class="fl">采样率</div>
          <div class="muted" style="font-size:12px">音频采样频率</div>
          <select v-model="sampleRate" class="sel" style="min-width:180px">
            <option v-for="o in rates" :key="o.v" :value="o.v">{{ o.label }}</option>
          </select>
        </div>

        <div class="modal-foot">
          <span class="muted" style="font-size:12px">将转换勾选的 {{ state.selected.size }} 首</span>
          <button class="ghost" @click="emit('close')">取消</button>
          <button :disabled="busy" @click="run">{{ busy ? '转换中…' : '确认执行' }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
