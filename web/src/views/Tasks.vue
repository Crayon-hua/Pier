<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { request, message, date, duration, statuses, kindNames, type Task } from '../api'
import TaskLog from '../components/TaskLog.vue'
const route = useRoute(),
  list = ref<Task[]>([]),
  loading = ref(true),
  error = ref(''),
  filter = ref('all'),
  search = ref(''),
  logID = ref(''),
  visible = ref(false)
const filtered = computed(() =>
  list.value.filter(
    (t) =>
      (filter.value === 'all' || t.status === filter.value) &&
      `${t.projectName} ${t.target}`.toLowerCase().includes(search.value.toLowerCase()),
  ),
)
async function load() {
  try {
    list.value = await request<Task[]>('/tasks')
    error.value = ''
  } catch (e) {
    error.value = message(e)
  } finally {
    loading.value = false
  }
}
function open(id: string) {
  logID.value = id
  visible.value = true
}
let timer: ReturnType<typeof setInterval>
onMounted(() => {
  void load()
  timer = setInterval(load, 3000)
  if (typeof route.query.id === 'string') open(route.query.id)
})
onUnmounted(() => clearInterval(timer))
</script>
<template>
  <div class="page-heading">
    <div>
      <span class="eyebrow">ACTIVITY & LOGS</span>
      <h1>任务记录<span class="heading-dot">.</span></h1>
      <p>追踪每一次部署、回退、备份和清理。展示最近 200 条任务。</p>
    </div>
    <a-button @click="load"
      ><template #icon><icon-refresh /></template>刷新</a-button
    >
  </div>
  <a-alert v-if="error" type="error" class="mb16">{{ error }}</a-alert>
  <section class="panel">
    <div class="toolbar padded">
      <a-select v-model="filter" style="width: 160px"
        ><a-option value="all">全部状态</a-option
        ><a-option v-for="(s, key) in statuses" :key="key" :value="key">{{
          s.label
        }}</a-option></a-select
      ><a-input-search
        v-model="search"
        placeholder="搜索项目或目标版本"
        allow-clear
        style="width: 260px"
      />
    </div>
    <a-table
      :data="filtered"
      :loading="loading"
      :bordered="false"
      :pagination="{ pageSize: 12 }"
      :scroll="{ x: 900 }"
      ><template #columns
        ><a-table-column title="项目"
          ><template #cell="{ record }"
            ><router-link :to="`/projects/${record.projectId}`" class="text-link">{{
              record.projectName
            }}</router-link></template
          ></a-table-column
        ><a-table-column title="操作"
          ><template #cell="{ record }">{{ kindNames[record.kind] }}</template></a-table-column
        ><a-table-column title="目标"
          ><template #cell="{ record }"
            ><span class="mono">{{ record.target || '—' }}</span></template
          ></a-table-column
        ><a-table-column title="状态"
          ><template #cell="{ record }"
            ><a-tag :color="statuses[record.status]?.color" size="small">{{
              statuses[record.status]?.label
            }}</a-tag></template
          ></a-table-column
        ><a-table-column title="耗时"
          ><template #cell="{ record }">{{ duration(record) }}</template></a-table-column
        ><a-table-column title="创建时间"
          ><template #cell="{ record }">{{ date(record.createdAt) }}</template></a-table-column
        ><a-table-column title="操作" :width="90"
          ><template #cell="{ record }"
            ><a-button type="text" @click="open(record.id)">查看日志</a-button></template
          ></a-table-column
        ></template
      ><template #empty><a-empty description="还没有任务记录" /></template
    ></a-table>
  </section>
  <TaskLog v-model:visible="visible" :task-id="logID" />
</template>
