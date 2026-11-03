<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  request,
  bytes,
  date,
  message,
  statuses,
  kindNames,
  typeNames,
  type Project,
  type Task,
  type System,
} from '../api'
import ProjectForm from '../components/ProjectForm.vue'
import TaskLog from '../components/TaskLog.vue'
const router = useRouter(),
  projects = ref<Project[]>([]),
  tasks = ref<Task[]>([]),
  system = ref<System>(),
  loading = ref(true),
  error = ref(''),
  create = ref(false),
  logVisible = ref(false),
  logID = ref('')
const deployed = computed(() => projects.value.filter((p) => p.state?.currentVersion).length)
const running = computed(
  () => tasks.value.filter((t) => ['running', 'queued'].includes(t.status)).length,
)
const disk = computed(() => projects.value.reduce((n, p) => n + p.diskBytes, 0))
async function load() {
  try {
    const [p, t, s] = await Promise.all([
      request<Project[]>('/projects'),
      request<Task[]>('/tasks'),
      request<System>('/system'),
    ])
    projects.value = p
    tasks.value = t
    system.value = s
    error.value = ''
  } catch (e) {
    error.value = message(e)
  } finally {
    loading.value = false
  }
}
let timer: ReturnType<typeof setInterval>
onMounted(() => {
  void load()
  timer = setInterval(load, 5000)
})
onUnmounted(() => clearInterval(timer))
function openLog(id: string) {
  logID.value = id
  logVisible.value = true
}
</script>
<template>
  <div class="page-heading">
    <div>
      <span class="eyebrow">WORKSPACE OVERVIEW</span>
      <h1>部署，一目了然<span class="heading-dot">.</span></h1>
      <p>你的应用、发布动态和服务器空间，都在这里。</p>
    </div>
    <a-button type="primary" size="large" @click="create = true"
      ><template #icon><icon-plus /></template>新建项目</a-button
    >
  </div>
  <a-alert v-if="error" type="error" class="mb16"
    >{{ error }} <a-button type="text" @click="load">重试</a-button></a-alert
  >
  <div class="stats-grid">
    <div class="stat-card">
      <span class="stat-label">全部项目 <icon-folder /></span>
      <div class="stat-number">{{ projects.length }}<small>个</small></div>
      <span class="stat-foot">独立管理，统一发布</span>
    </div>
    <div class="stat-card">
      <span class="stat-label">已部署项目 <icon-check-circle /></span>
      <div class="stat-number">
        {{ deployed }}<small>/ {{ projects.length }}</small>
      </div>
      <span class="stat-foot"><i class="tiny-dot green"></i> 按本机部署状态统计</span>
    </div>
    <div class="stat-card">
      <span class="stat-label">执行中任务 <icon-sync /></span>
      <div class="stat-number">{{ running }}<small>个</small></div>
      <span class="stat-foot">{{ running ? '发布正在进行' : '当前没有进行中的任务' }}</span>
    </div>
    <div class="stat-card">
      <span class="stat-label">项目空间占用 <icon-storage /></span>
      <div class="stat-number disk-number">{{ bytes(disk) }}</div>
      <span class="stat-foot">版本、缓存和备份</span>
    </div>
  </div>
  <div class="overview-columns">
    <section class="panel">
      <div class="panel-header">
        <h3>
          我的项目 <span class="count">{{ projects.length }}</span>
        </h3>
        <router-link to="/projects" class="text-link">全部项目 <icon-arrow-right /></router-link>
      </div>
      <a-spin :loading="loading" class="full-width"
        ><div v-if="!projects.length" class="empty-state">
          <span class="empty-icon"><icon-folder-add /></span>
          <h3>为第一个应用搭座栈桥</h3>
          <p>添加项目，从 GitHub Release 或本地包开始发布。</p>
          <a-button type="primary" @click="create = true">创建项目</a-button>
        </div>
        <div v-else class="project-list">
          <router-link
            v-for="p in projects.slice(0, 6)"
            :key="p.id"
            :to="`/projects/${p.id}`"
            class="project-row"
            ><span :class="['project-icon', p.type]"
              ><icon-code-square v-if="p.type === 'static'" /><icon-common
                v-else-if="p.type === 'docker'" /><icon-desktop v-else
            /></span>
            <div class="project-row-main">
              <strong>{{ p.name }}</strong
              ><span>{{ typeNames[p.type] }} <i>·</i> {{ p.repository || '本地部署包' }}</span>
            </div>
            <div class="project-row-state">
              <span class="mono">{{ p.state?.currentVersion || '尚未部署' }}</span
              ><small
                ><i
                  :class="[
                    'tiny-dot',
                    p.busy ? 'blue' : p.state?.currentVersion ? 'green' : 'gray',
                  ]"
                ></i
                >{{ p.busy ? '执行中' : p.state?.currentVersion ? '已部署' : '待发布' }}</small
              >
            </div>
            <icon-right
          /></router-link></div
      ></a-spin>
    </section>
    <section class="panel host-panel">
      <div class="panel-header">
        <h3>当前主机</h3>
        <a-tag size="small" color="arcoblue">LOCAL</a-tag>
      </div>
      <div class="host-illustration"><icon-desktop /><span class="connection-dot"></span></div>
      <h3>本地工作节点</h3>
      <p class="muted">{{ system ? `${system.os} / ${system.arch}` : '正在连接…' }}</p>
      <div class="host-facts">
        <span>运行模式<strong>单机部署</strong></span
        ><span
          >控制台版本<strong>v{{ system?.version || '0.1.0' }}</strong></span
        ><span>部署引擎<strong>upy core</strong></span>
      </div>
      <div class="host-note"><icon-info-circle /> 已部署表示发布记录成功，不代表实时健康状态。</div>
    </section>
  </div>
  <section class="panel">
    <div class="panel-header">
      <h3>最近任务</h3>
      <router-link to="/tasks" class="text-link">查看记录 <icon-arrow-right /></router-link>
    </div>
    <a-table
      :data="tasks.slice(0, 5)"
      :pagination="false"
      :bordered="false"
      :loading="loading"
      :scroll="{ x: 700 }"
      ><template #columns
        ><a-table-column title="项目" data-index="projectName" /><a-table-column title="操作"
          ><template #cell="{ record }"
            ><span
              >{{ kindNames[record.kind] }}
              <span class="muted mono">{{ record.target }}</span></span
            ></template
          ></a-table-column
        ><a-table-column title="状态"
          ><template #cell="{ record }"
            ><a-tag :color="statuses[record.status]?.color" size="small">{{
              statuses[record.status]?.label
            }}</a-tag></template
          ></a-table-column
        ><a-table-column title="时间"
          ><template #cell="{ record }">{{ date(record.createdAt) }}</template></a-table-column
        ><a-table-column title="" :width="88"
          ><template #cell="{ record }"
            ><a-button type="text" @click="openLog(record.id)">日志</a-button></template
          ></a-table-column
        ></template
      ><template #empty><a-empty description="发布、备份和清理记录将在这里出现" /></template
    ></a-table>
  </section>
  <ProjectForm v-model:visible="create" @saved="load" /><TaskLog
    v-model:visible="logVisible"
    :task-id="logID"
  />
</template>
