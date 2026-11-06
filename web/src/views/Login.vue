<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { message, post, request } from '../api'
import { auth } from '../stores/auth'
const router = useRouter(),
  initialized = ref(true),
  ready = ref(false),
  loading = ref(false),
  error = ref('')
const form = reactive({ username: '', password: '', confirm: '' })
async function load() {
  error.value = ''
  try {
    initialized.value = (await request<{ initialized: boolean }>('/auth/status')).initialized
    ready.value = true
  } catch (e) {
    error.value = message(e)
  }
}
onMounted(load)
async function submit() {
  error.value = ''
  if (!form.username || !form.password) {
    error.value = '请填写用户名和密码'
    return
  }
  if (!initialized.value && form.password !== form.confirm) {
    error.value = '两次输入的密码不一致'
    return
  }
  loading.value = true
  try {
    if (!initialized.value) {
      await post('/auth/setup', { username: form.username, password: form.password })
      initialized.value = true
      Message.success('管理员已创建')
    }
    const user = await post<{ username: string }>('/auth/login', {
      username: form.username,
      password: form.password,
    })
    auth.username = user.username
    auth.checked = true
    await router.replace('/')
  } catch (e) {
    error.value = message(e)
  } finally {
    loading.value = false
  }
}
</script>
<template>
  <div class="login-page">
    <section class="login-story">
      <div class="brand light">
        <span class="brand-symbol">P<span></span></span><span>Pier <small>栈桥</small></span>
      </div>
      <div class="story-content">
        <span class="eyebrow">单机应用部署控制台</span>
        <h1>每次发布，<br />都有据可循</h1>
        <p>
          从部署包到运行版本，把应用发布、版本回退<br
            class="desktop-break"
          />与日常维护放在同一个工作台
        </p>
        <div class="deploy-route" aria-label="发布流程：部署包、发布、健康检查">
          <div class="route-caption">
            <span>一次发布的路径</span><span class="mono">Pier / deploy</span>
          </div>
          <ol>
            <li>
              <span class="route-number">01</span
              ><span><strong>选择部署包</strong><small>GitHub Release / 本地上传</small></span
              ><icon-file />
            </li>
            <li>
              <span class="route-number">02</span
              ><span><strong>发布到应用</strong><small>静态站点 / Docker / 系统服务</small></span
              ><icon-send />
            </li>
            <li>
              <span class="route-number">03</span
              ><span><strong>检查与回退</strong><small>健康检查失败时自动尝试回滚</small></span
              ><icon-check-circle />
            </li>
          </ol>
        </div>
      </div>
      <div class="story-footer">一个工作台 · 一台服务器 · 多个应用</div>
    </section>
    <section class="login-form-side">
      <div class="login-form">
        <span class="eyebrow">Pier 工作空间</span>
        <h2>{{ initialized ? '登录工作台' : '创建你的工作空间' }}</h2>
        <p class="muted">
          {{ initialized ? '使用管理员账户继续管理应用' : '首次使用，设置管理员用户名和密码' }}
        </p>
        <a-alert v-if="error" type="error" class="mb16">{{ error }}</a-alert>
        <a-form v-if="ready" :model="form" layout="vertical" @submit-success="submit"
          ><a-form-item
            label="用户名"
            field="username"
            :rules="[{ required: true, message: '请输入用户名' }]"
            ><a-input
              v-model="form.username"
              size="large"
              placeholder="输入管理员用户名"
              autocomplete="username"
              ><template #prefix><icon-user /></template></a-input></a-form-item
          ><a-form-item
            label="密码"
            field="password"
            :rules="[{ required: true, message: '请输入密码' }]"
            ><a-input-password
              v-model="form.password"
              size="large"
              :placeholder="initialized ? '输入密码' : '至少 12 字节的密码'"
              :autocomplete="initialized ? 'current-password' : 'new-password'" /></a-form-item
          ><a-form-item v-if="!initialized" label="确认密码"
            ><a-input-password
              v-model="form.confirm"
              size="large"
              placeholder="再次输入密码"
              autocomplete="new-password" /></a-form-item
          ><a-button html-type="submit" type="primary" size="large" long :loading="loading"
            >{{ initialized ? '登录工作台' : '创建管理员并登录'
            }}<template #icon><icon-arrow-right /></template></a-button
        ></a-form>
        <a-button v-else @click="load">重新连接</a-button>
        <div class="login-note"><icon-safe /> 仅管理员可以访问此工作空间</div>
      </div>
      <span class="login-copyright">Pier 栈桥 · v0.1.0</span>
    </section>
  </div>
</template>
