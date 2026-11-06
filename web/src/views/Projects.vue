<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { request, typeNames, bytes, date, message, type Project } from '../api'
import ProjectForm from '../components/ProjectForm.vue'
const list = ref<Project[]>([]),
  loading = ref(true),
  error = ref(''),
  search = ref(''),
  filter = ref('all'),
  create = ref(false)
const filtered = computed(() =>
  list.value.filter(
    (p) =>
      (filter.value === 'all' || p.type === filter.value) &&
      `${p.name} ${p.id} ${p.repository}`.toLowerCase().includes(search.value.toLowerCase()),
  ),
)
async function load() {
  loading.value = true
  try {
    list.value = await request<Project[]>('/projects')
    error.value = ''
  } catch (e) {
    error.value = message(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>
<template>
  <div class="page-heading">
    <div>
      <span class="eyebrow">应用管理</span>
      <h1>项目</h1>
      <p>每个应用一个独立空间，让版本和数据各归其位。</p>
    </div>
    <a-button type="primary" size="large" @click="create = true"
      ><template #icon><icon-plus /></template>新建项目</a-button
    >
  </div>
  <a-alert v-if="error" type="error" class="mb16">{{ error }}</a-alert>
  <div class="toolbar">
    <a-radio-group v-model="filter" type="button"
      ><a-radio value="all">全部 {{ list.length }}</a-radio
      ><a-radio value="static">静态站点</a-radio><a-radio value="docker">Docker</a-radio
      ><a-radio value="system">系统服务</a-radio></a-radio-group
    >
    <div class="toolbar-right">
      <a-input-search
        v-model="search"
        placeholder="搜索项目或仓库"
        allow-clear
        style="width: 240px"
      /><a-button aria-label="刷新项目" @click="load"><icon-refresh /></a-button>
    </div>
  </div>
  <a-spin :loading="loading" class="full-width"
    ><div v-if="filtered.length" class="project-grid">
      <router-link v-for="p in filtered" :key="p.id" :to="`/projects/${p.id}`" class="project-card"
        ><div class="project-card-top">
          <span :class="['project-icon', p.type]"
            ><icon-code-square v-if="p.type === 'static'" /><icon-common
              v-else-if="p.type === 'docker'" /><icon-desktop v-else /></span
          ><a-tag
            :color="p.busy ? 'arcoblue' : p.state?.currentVersion ? 'green' : 'gray'"
            size="small"
            >{{ p.busy ? '执行中' : p.state?.currentVersion ? '已部署' : '待发布' }}</a-tag
          >
        </div>
        <h3>{{ p.name }}</h3>
        <p class="project-description">{{ p.description || '还没有项目描述' }}</p>
        <div class="project-repository"><icon-github /> {{ p.repository || '本地部署包' }}</div>
        <div class="project-card-facts">
          <span
            >{{ typeNames[p.type]
            }}<strong class="mono">{{ p.state?.currentVersion || '—' }}</strong></span
          ><span
            >空间占用<strong>{{ bytes(p.diskBytes) }}</strong></span
          >
        </div>
        <div class="project-card-footer">
          <span>{{ p.state?.deployedAt ? date(p.state.deployedAt) : '等待第一次发布' }}</span
          ><icon-arrow-right /></div
      ></router-link>
    </div>
    <section v-else class="panel empty-state tall">
      <span class="empty-icon"><icon-folder-add /></span>
      <h3>{{ list.length ? '没有匹配的项目' : '从你的第一个项目开始' }}</h3>
      <p>
        {{
          list.length ? '试试其他关键词或部署类型。' : '支持静态网站、Docker 和 systemd 原生服务。'
        }}
      </p>
      <a-button v-if="!list.length" type="primary" @click="create = true">新建项目</a-button>
    </section></a-spin
  ><ProjectForm v-model:visible="create" @saved="load" />
</template>
