<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { request, post, message, type System } from '../api'
import { auth } from '../stores/auth'
const system = ref<System>(),
  error = ref(''),
  saving = ref(false),
  router = useRouter()
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })
onMounted(async () => {
  try {
    system.value = await request<System>('/system')
  } catch (e) {
    error.value = message(e)
  }
})
async function save() {
  if (form.newPassword !== form.confirm) {
    Message.error('两次密码不一致')
    return
  }
  saving.value = true
  try {
    await post('/auth/password', { oldPassword: form.oldPassword, newPassword: form.newPassword })
    auth.username = ''
    Message.success('密码已更新，请重新登录')
    await router.push('/login')
  } catch (e) {
    Message.error(message(e))
  } finally {
    saving.value = false
  }
}
</script>
<template>
  <div class="page-heading">
    <div>
      <span class="eyebrow">工作空间</span>
      <h1>设置</h1>
      <p>管理你的登录凭据，查看工作空间运行信息。</p>
    </div>
  </div>
  <a-alert v-if="error" type="error" class="mb16">{{ error }}</a-alert>
  <div class="settings-grid">
    <section class="panel settings-panel">
      <h3><icon-user /> 管理员账户</h3>
      <p class="muted">当前用户：{{ auth.username }}。修改密码后所有登录会话将失效。</p>
      <a-form :model="form" layout="vertical" @submit-success="save"
        ><a-form-item
          field="oldPassword"
          label="当前密码"
          :rules="[{ required: true, message: '请输入当前密码' }]"
          ><a-input-password
            v-model="form.oldPassword"
            autocomplete="current-password" /></a-form-item
        ><a-form-item
          field="newPassword"
          label="新密码"
          :rules="[{ required: true, minLength: 12, message: '密码至少 12 个字符' }]"
          ><a-input-password
            v-model="form.newPassword"
            placeholder="至少 12 字节，最多 72 字节"
            autocomplete="new-password" /></a-form-item
        ><a-form-item
          field="confirm"
          label="确认新密码"
          :rules="[{ required: true, message: '请确认新密码' }]"
          ><a-input-password v-model="form.confirm" autocomplete="new-password" /></a-form-item
        ><a-button type="primary" html-type="submit" :loading="saving">更新密码</a-button></a-form
      >
    </section>
    <section class="panel settings-panel">
      <h3><icon-desktop /> 工作空间信息</h3>
      <a-descriptions :column="1" :label-style="{ width: '110px' }"
        ><a-descriptions-item label="版本"
          >Pier v{{ system?.version || '0.1.0' }}</a-descriptions-item
        ><a-descriptions-item label="运行平台"
          >{{ system?.os }} / {{ system?.arch }}</a-descriptions-item
        ><a-descriptions-item label="数据目录"
          ><span class="mono break-all">{{ system?.dataDir }}</span></a-descriptions-item
        ><a-descriptions-item label="上传上限">{{ system?.maxUploadMB }} MB</a-descriptions-item
        ><a-descriptions-item label="部署模式">单机</a-descriptions-item></a-descriptions
      ><a-divider />
      <h4>支持的部署方式</h4>
      <p class="muted">静态网站 · Docker / Compose · systemd 服务</p>
      <p class="muted small">
        Docker 部署需要主机安装 Docker；systemd 服务需要 Linux 及对应操作权限。备份执行项目内的
        backup.sh 脚本。
      </p>
    </section>
  </div>
</template>
