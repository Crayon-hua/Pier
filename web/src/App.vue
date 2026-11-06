<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { auth, logout } from './stores/auth'
const route = useRoute(),
  router = useRouter()
const selected = computed(() => (route.path.startsWith('/projects') ? '/projects' : route.path))
function navigate(key: string) {
  void router.push(key)
}
async function signOut() {
  try {
    await logout()
    await router.push('/login')
  } catch {
    Message.error('退出失败，请重试')
  }
}
</script>

<template>
  <router-view v-if="route.meta.public" />
  <div v-else class="app-shell">
    <aside class="sidebar">
      <router-link to="/" class="brand"
        ><span class="brand-symbol">P<span></span></span
        ><span>Pier <small>栈桥</small></span></router-link
      >
      <div class="workspace-label">工作空间 <span>单机</span></div>
      <a-menu aria-label="主导航" :selected-keys="[selected]" @menu-item-click="navigate">
        <a-menu-item key="/"
          ><template #icon><icon-apps /></template>概览</a-menu-item
        >
        <a-menu-item key="/projects"
          ><template #icon><icon-folder /></template>项目</a-menu-item
        >
        <a-menu-item key="/tasks"
          ><template #icon><icon-history /></template>任务记录</a-menu-item
        >
        <a-menu-item key="/settings"
          ><template #icon><icon-settings /></template>设置</a-menu-item
        >
      </a-menu>
      <div class="sidebar-bottom">
        <span class="connection-dot"></span> 本机部署控制台<small>Pier / v0.1.0</small>
      </div>
    </aside>
    <div class="main-shell">
      <header class="topbar">
        <div class="breadcrumb">
          工作空间 <icon-right /> <span>{{ route.meta.title }}</span>
        </div>
        <div class="topbar-right">
          <span class="local-label"><icon-desktop /> 本机</span
          ><span class="avatar">{{ auth.username.slice(0, 1).toUpperCase() }}</span
          ><span>{{ auth.username }}</span
          ><a-button type="text" size="small" aria-label="退出登录" @click="signOut"
            ><icon-export
          /></a-button>
        </div>
      </header>
      <main class="page"><router-view /></main>
      <footer class="page-footer">Pier 栈桥 <span>单机应用部署控制台</span></footer>
    </div>
  </div>
</template>
