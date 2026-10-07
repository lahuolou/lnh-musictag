<script setup>
import { ref } from 'vue'
import { state, toast, logout, go } from '../store.js'
import { api } from '../api.js'

const oldPass = ref('')
const newPass = ref('')
const confirm = ref('')
const msg = ref('')

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
</script>

<template>
  <div>
    <header>
      <h1>⚙️ LNH-MusicTag 设置</h1>
      <span class="badge">后台账号：{{ state.user }}</span>
      <span class="navbar"></span>
      <button class="navlink" @click="go('/')">← 返回主页</button>
      <button class="ghost sm" @click="logout">退出</button>
    </header>
    <div class="wrap">
      <div class="grid2" style="align-items:start">
        <div class="panel">
          <h2>🔒 修改密码</h2>
          <p class="muted" style="margin:0 0 10px">修改后台登录密码（至少 6 位）。修改后立即生效，下次登录请用新密码。</p>
          <div class="field"><label>旧密码</label><input type="password" v-model="oldPass" autocomplete="current-password" /></div>
          <div class="field"><label>新密码</label><input type="password" v-model="newPass" autocomplete="new-password" /></div>
          <div class="field"><label>确认新密码</label><input type="password" v-model="confirm" autocomplete="new-password" @keydown.enter="doChange" /></div>
          <div class="row" style="margin-top:8px">
            <button @click="doChange">确认修改</button>
            <button class="ghost" @click="go('/')">返回主页</button>
          </div>
          <div class="loading login-msg">{{ msg }}</div>
        </div>
        <div>
          <div class="panel">
            <h2>ℹ️ 后台信息</h2>
            <p class="muted">登录账号：<b>{{ state.user }}</b></p>
            <p class="muted">指纹引擎：<b>{{ state.fpActive ? '已启用' : '未启用' }}</b>（批量刮削/重复检测参考）</p>
            <p class="muted">部署形态：纯 Docker · 端口 10248 · 配置卷持久化</p>
          </div>
          <div class="panel">
            <h2>💡 提示</h2>
            <p class="muted">密码修改不会强制进行，可随时进入本页设置；<br>首次启动若使用随机初始密码，建议尽快在此更换。</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
