<script setup>
import { onMounted, ref } from 'vue'
import { state, toast } from '../store.js'
import { api } from '../api.js'

const oldPass = ref('')
const newPass = ref('')
const confirm = ref('')
const msg = ref('')

const cfg = ref({ adminUser: '', acoustidKey: '', autoFixTitle: true, autoRenameFile: true, scanDir: '' })
const cfgMsg = ref('')

async function doChange() {
  msg.value = '';
  if (newPass.value !== confirm.value) { msg.value = '两次输入的新密码不一致'; return; }
  if (newPass.value.length < 6) { msg.value = '新密码至少 6 位'; return; }
  try {
    await api('/api/change-password', { method: 'POST', body: JSON.stringify({ oldPass: oldPass.value, newPass: newPass.value }) });
    toast('密码已修改', 'ok');
    oldPass.value = newPass.value = confirm.value = '';
    msg.value = '';
  } catch (e) { msg.value = e.message; }
}

async function loadCfg() {
  try { cfg.value = await api('/api/settings'); }
  catch (e) { toast('读取配置失败: ' + e.message, 'err'); }
}

async function saveCfg() {
  cfgMsg.value = '';
  try {
    await api('/api/settings', {
      method: 'POST',
      body: JSON.stringify({
        adminUser: cfg.value.adminUser,
        acoustidKey: cfg.value.acoustidKey,
        autoFixTitle: cfg.value.autoFixTitle,
        autoRenameFile: cfg.value.autoRenameFile,
        scanDir: cfg.value.scanDir
      })
    });
    toast('配置已保存', 'ok');
    cfgMsg.value = '已保存到数据库（重启后仍生效）。';
  } catch (e) { cfgMsg.value = '保存失败: ' + e.message; }
}

onMounted(loadCfg);
</script>

<template>
  <div class="grid2" style="align-items:start">
    <div>
      <div class="panel">
        <h2>🔒 修改密码</h2>
        <p class="muted" style="margin:0 0 10px">修改后台登录密码（至少 6 位），保存到数据库，立即生效。</p>
        <div class="field"><label>旧密码</label><input type="password" v-model="oldPass" autocomplete="current-password" /></div>
        <div class="field"><label>新密码</label><input type="password" v-model="newPass" autocomplete="new-password" /></div>
        <div class="field"><label>确认新密码</label><input type="password" v-model="confirm" autocomplete="new-password" @keydown.enter="doChange" /></div>
        <div class="row">
          <button @click="doChange">确认修改</button>
        </div>
        <div class="loading login-msg">{{ msg }}</div>
      </div>

      <div class="panel">
        <h2>💡 提示</h2>
        <p class="muted">账号、密码、API Key、选项均保存在 <b>SQLite 数据库</b>（/config/lnh.db），不再使用环境变量；<br>
        首次启动若为随机初始密码，建议尽快在左侧修改。</p>
      </div>
    </div>

    <div class="panel">
      <h2>⚙️ 数据库配置</h2>
      <div class="field">
        <label>后台账号 admin_user</label>
        <input type="text" v-model="cfg.adminUser" />
      </div>
      <div class="field">
        <label>AcoustID API Key（指纹识别，可留空）</label>
        <input type="text" v-model="cfg.acoustidKey" placeholder="可选" />
      </div>
      <div class="field">
        <label>默认音乐目录（容器内路径）</label>
        <input type="text" v-model="cfg.scanDir" placeholder="/music" />
      </div>
      <div class="chk" style="margin:6px 0 14px">
        <input type="checkbox" id="autoFix" class="tchk" v-model="cfg.autoFixTitle" />
        <label for="autoFix">扫描时默认去除标题后缀（广岛之恋.mp3 → 广岛之恋）</label>
      </div>
      <div class="chk" style="margin:0 0 14px">
        <input type="checkbox" id="autoRename" class="tchk" v-model="cfg.autoRenameFile" />
        <label for="autoRename">扫描时自动重命名文件：去重复/错误音频后缀（发如雪.mp3.flac → 发如雪.flac，先校验真实编码格式）</label>
      </div>
      <div class="row">
        <button @click="saveCfg">保存配置</button>
      </div>
      <div class="loading login-msg">{{ cfgMsg }}</div>
    </div>
  </div>
</template>
