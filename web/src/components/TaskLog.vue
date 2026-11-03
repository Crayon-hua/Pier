<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { request, message, statuses, kindNames, date, duration, type Task } from '../api'
const props = defineProps<{ taskId: string; visible: boolean }>()
const emit = defineEmits<{ 'update:visible': [value: boolean] }>()
const task = ref<Task>(),
  text = ref(''),
  error = ref(''),
  loading = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
const plain = computed(() => text.value.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, ''))
let generation = 0
async function load(g: number) {
  if (!props.visible) return
  loading.value = true
  try {
    const [t, logs] = await Promise.all([
      request<Task>(`/tasks/${props.taskId}`),
      request<{ text: string }>(`/tasks/${props.taskId}/logs`),
    ])
    if (g !== generation) return
    task.value = t
    text.value = logs.text
    error.value = ''
    if (['running', 'queued'].includes(t.status)) timer = setTimeout(() => void load(g), 1500)
  } catch (e) {
    if (g === generation) error.value = message(e)
  } finally {
    if (g === generation) loading.value = false
  }
}
watch(
  () => [props.visible, props.taskId],
  () => {
    generation++
    clearTimeout(timer)
    task.value = undefined
    text.value = ''
    if (props.visible) void load(generation)
  },
)
onUnmounted(() => {
  generation++
  clearTimeout(timer)
})
function download() {
  const blob = new Blob([plain.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `pier-${props.taskId}.log`
  a.click()
  URL.revokeObjectURL(url)
}
</script>
<template>
  <a-drawer
    :visible="visible"
    :width="'min(900px, 100vw)'"
    title="任务详情"
    :footer="false"
    @cancel="emit('update:visible', false)"
    ><template v-if="task"
      ><div class="section-heading">
        <div>
          <h2>
            {{ task.projectName }}
            <a-tag :color="statuses[task.status]?.color">{{
              statuses[task.status]?.label || task.status
            }}</a-tag>
          </h2>
          <p class="muted">
            {{ kindNames[task.kind] }} · {{ task.target || '默认' }} · {{ duration(task) }}
          </p>
        </div>
        <a-button @click="download"
          ><template #icon><icon-download /></template>下载日志</a-button
        >
      </div>
      <div class="task-meta">
        <span>开始时间 {{ date(task.startedAt) }}</span
        ><span class="mono">{{ task.id }}</span>
      </div>
      <a-alert v-if="task.error" type="warning" class="mb16">{{ task.error }}</a-alert></template
    ><a-alert v-if="error" type="error" class="mb16"
      >{{ error }} <a-button type="text" @click="load(generation)">重试</a-button></a-alert
    >
    <div class="terminal">
      <div class="terminal-bar">
        <span></span><span></span><span></span
        ><label>执行日志 {{ loading ? '· 正在同步' : '' }}</label>
      </div>
      <pre>{{ plain || '等待日志…' }}</pre>
    </div></a-drawer
  >
</template>
