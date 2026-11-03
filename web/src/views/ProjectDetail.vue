<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import {
  request,
  post,
  bytes,
  date,
  duration,
  message,
  typeNames,
  kindNames,
  statuses,
  type Project,
  type Task,
  type Release,
} from '../api'
import ProjectForm from '../components/ProjectForm.vue'
import TaskLog from '../components/TaskLog.vue'
const route = useRoute(),
  router = useRouter(),
  id = computed(() => String(route.params.id))
const project = ref<Project>(),
  tasks = ref<Task[]>([]),
  error = ref(''),
  loading = ref(true),
  edit = ref(false),
  deployVisible = ref(false),
  logVisible = ref(false),
  logID = ref(''),
  tab = ref('versions')
const source = ref('release'),
  target = ref('latest'),
  force = ref(false),
  file = ref<File>(),
  releasing = ref(false),
  releases = ref<Release[]>([]),
  releaseError = ref(''),
  releaseLoading = ref(false),
  actionError = ref('')
const selectedFile = ref<HTMLInputElement>()
const reclaimable = computed(
  () =>
    (project.value?.cleanup?.BundleBytes || 0) +
    (project.value?.cleanup?.OldReleaseBytes || 0) +
    (project.value?.cleanup?.TmpBytes || 0),
)
async function load() {
  const currentID = id.value
  try {
    const [p, t] = await Promise.all([
      request<Project>(`/projects/${currentID}`),
      request<Task[]>(`/tasks?projectId=${currentID}`),
    ])
    if (currentID !== id.value) return
    project.value = p
    tasks.value = t
    error.value = ''
  } catch (e) {
    error.value = message(e)
  } finally {
    loading.value = false
  }
}
watch(
  id,
  () => {
    project.value = undefined
    loading.value = true
    void load()
  },
  { immediate: true },
)
let timer: ReturnType<typeof setInterval>
onMounted(() => {
  timer = setInterval(() => {
    if (project.value?.busy) void load()
  }, 2500)
})
onUnmounted(() => clearInterval(timer))
function openLog(t: Task) {
  logID.value = t.id
  logVisible.value = true
}
function prepareDeploy() {
  source.value = project.value?.repository ? 'release' : 'local'
  target.value = 'latest'
  force.value = false
  file.value = undefined
  actionError.value = ''
  deployVisible.value = true
  releases.value = []
  releaseError.value = ''
}
async function loadReleases() {
  releaseLoading.value = true
  releaseError.value = ''
  try {
    releases.value = (await request<Release[]>(`/projects/${id.value}/releases`)) || []
  } catch (e) {
    releaseError.value = message(e)
  } finally {
    releaseLoading.value = false
  }
}
async function deploy() {
  releasing.value = true
  actionError.value = ''
  try {
    let task: Task
    if (source.value === 'local') {
      if (!file.value) throw new Error('请选择一个部署包')
      const data = new FormData()
      data.append('file', file.value)
      data.append('force', String(force.value))
      task = await request<Task>(`/projects/${id.value}/upload`, { method: 'POST', body: data })
    } else
      task = await post<Task>(`/projects/${id.value}/tasks`, {
        kind: 'deploy',
        target: target.value || 'latest',
        force: force.value,
      })
    deployVisible.value = false
    openLog(task)
    await load()
  } catch (e) {
    actionError.value = message(e)
  } finally {
    releasing.value = false
  }
}
function act(kind: string, target = '', description = '') {
  Modal.confirm({
    title:
      kind === 'rollback'
        ? `回退到 ${target}？`
        : kind === 'backup'
          ? '执行项目备份？'
          : '清理项目数据？',
    content: description || '操作将在后台执行，可在任务日志中查看结果。',
    okText: '确认执行',
    cancelText: '取消',
    onBeforeOk: async () => {
      try {
        const t = await post<Task>(`/projects/${id.value}/tasks`, { kind, target })
        openLog(t)
        await load()
        return true
      } catch (e) {
        Message.error(message(e))
        return false
      }
    },
  })
}
function remove() {
  Modal.confirm({
    title: '移除项目登记？',
    content:
      '部署产物和持久化数据将保留在磁盘上，任务记录也会保留。移除后无法通过控制台管理此项目。',
    okButtonProps: { status: 'danger' },
    onBeforeOk: async () => {
      try {
        await request(`/projects/${id.value}`, { method: 'DELETE' })
        await router.push('/projects')
        return true
      } catch (e) {
        Message.error(message(e))
        return false
      }
    },
  })
}
function choose(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0]
}
</script>
<template>
  <a-button type="text" class="back-link" @click="router.push('/projects')"
    ><template #icon><icon-arrow-left /></template>返回项目</a-button
  >
  <a-alert v-if="error" type="error" class="mb16"
    >{{ error }} <a-button type="text" @click="load">重试</a-button></a-alert
  >
  <a-spin :loading="loading" class="full-width"
    ><template v-if="project">
      <div class="page-heading detail-heading">
        <div class="detail-title">
          <span :class="['project-icon large', project.type]"
            ><icon-code-square v-if="project.type === 'static'" /><icon-common
              v-else-if="project.type === 'docker'" /><icon-desktop v-else
          /></span>
          <div>
            <div class="inline-title">
              <h1>{{ project.name }}</h1>
              <a-tag size="small">{{ typeNames[project.type] }}</a-tag>
            </div>
            <p>{{ project.description || project.id }}</p>
          </div>
        </div>
        <a-space
          ><a-button :disabled="project.busy" @click="edit = true"
            ><template #icon><icon-settings /></template>项目设置</a-button
          ><a-button type="primary" :disabled="project.busy" @click="prepareDeploy"
            ><template #icon><icon-send /></template
            >{{ project.busy ? '任务执行中' : '发布新版本' }}</a-button
          ></a-space
        >
      </div>
      <a-alert v-if="project.stateError" type="warning" class="mb16"
        >部署状态读取失败：{{ project.stateError }}</a-alert
      >
      <a-alert v-if="project.busy" type="info" class="mb16"
        >项目任务正在执行，页面将自动同步结果。<a-button
          type="text"
          @click="openLog(tasks.find((t) => ['running', 'queued'].includes(t.status))!)"
          >查看日志</a-button
        ></a-alert
      >
      <div class="detail-summary panel">
        <div>
          <span class="stat-label">当前版本</span
          ><strong class="mono">{{ project.state?.currentVersion || '尚未部署' }}</strong>
        </div>
        <div>
          <span class="stat-label">上一个版本</span
          ><strong class="mono">{{ project.state?.previousVersion || '—' }}</strong>
        </div>
        <div>
          <span class="stat-label">最近部署</span
          ><strong>{{ date(project.state?.deployedAt) }}</strong>
        </div>
        <div>
          <span class="stat-label">空间占用</span><strong>{{ bytes(project.diskBytes) }}</strong>
        </div>
      </div>
      <div class="project-path">
        <icon-folder /> <span class="mono">{{ project.root }}</span
        ><span v-if="project.repository" class="repository-path"
          ><icon-github /> {{ project.repository }}</span
        >
      </div>
      <section class="panel detail-tabs">
        <a-tabs v-model:active-key="tab">
          <a-tab-pane key="versions" title="版本管理"
            ><div class="tab-intro">
              <div>
                <h3>本地版本</h3>
                <p class="muted">
                  默认保留当前版本与上一版本。回退会重新部署所选版本并执行健康检查。
                </p>
              </div>
              <a-button @click="load"><icon-refresh /></a-button>
            </div>
            <a-alert v-if="project.versionsError" type="error">{{ project.versionsError }}</a-alert
            ><a-table
              :data="project.versions || []"
              :bordered="false"
              :pagination="false"
              :scroll="{ x: 650 }"
              ><template #columns
                ><a-table-column title="版本"
                  ><template #cell="{ record }"
                    ><strong class="mono">{{ record.version }}</strong>
                    <a-tag
                      v-if="record.version === project.state?.currentVersion"
                      color="green"
                      size="small"
                      >当前</a-tag
                    ><a-tag
                      v-else-if="record.version === project.state?.previousVersion"
                      size="small"
                      >上一版本</a-tag
                    ></template
                  ></a-table-column
                ><a-table-column title="安装时间"
                  ><template #cell="{ record }">{{
                    date(record.createdAt)
                  }}</template></a-table-column
                ><a-table-column title="大小"
                  ><template #cell="{ record }">{{ bytes(record.bytes) }}</template></a-table-column
                ><a-table-column title="操作" :width="100"
                  ><template #cell="{ record }"
                    ><a-button
                      type="text"
                      :disabled="project.busy || record.version === project.state?.currentVersion"
                      @click="
                        act(
                          'rollback',
                          record.version,
                          '将切换服务到这个版本。涉及数据结构变更时，请先确认应用兼容性。',
                        )
                      "
                      >回退</a-button
                    ></template
                  ></a-table-column
                ></template
              ><template #empty
                ><a-empty description="还没有版本，发布第一个部署包吧" /></template></a-table
          ></a-tab-pane>
          <a-tab-pane key="tasks" title="任务记录"
            ><div class="tab-intro">
              <div>
                <h3>项目动态</h3>
                <p class="muted">每次操作都有独立记录与执行日志。</p>
              </div>
            </div>
            <a-table
              :data="tasks"
              :bordered="false"
              :pagination="{ pageSize: 8 }"
              :scroll="{ x: 650 }"
              ><template #columns
                ><a-table-column title="操作"
                  ><template #cell="{ record }"
                    >{{ kindNames[record.kind] }}
                    <span class="muted mono">{{ record.target }}</span></template
                  ></a-table-column
                ><a-table-column title="状态"
                  ><template #cell="{ record }"
                    ><a-tag :color="statuses[record.status]?.color" size="small">{{
                      statuses[record.status]?.label
                    }}</a-tag></template
                  ></a-table-column
                ><a-table-column title="耗时"
                  ><template #cell="{ record }">{{ duration(record) }}</template></a-table-column
                ><a-table-column title="时间"
                  ><template #cell="{ record }">{{
                    date(record.createdAt)
                  }}</template></a-table-column
                ><a-table-column title="" :width="90"
                  ><template #cell="{ record }"
                    ><a-button type="text" @click="openLog(record)">日志</a-button></template
                  ></a-table-column
                ></template
              ></a-table
            ></a-tab-pane
          >
          <a-tab-pane key="backups" title="备份归档"
            ><div class="tab-intro">
              <div>
                <h3>数据备份</h3>
                <p class="muted">
                  执行项目内 backup.sh，自动轮转保留归档。脚本通过 UPY_BACKUP_DIR 获取输出路径。
                </p>
              </div>
              <a-button type="primary" :disabled="project.busy" @click="act('backup')"
                ><template #icon><icon-save /></template>立即备份</a-button
              >
            </div>
            <a-alert v-if="project.backupsError" type="error">{{ project.backupsError }}</a-alert
            ><a-table :data="project.backups || []" :bordered="false" :pagination="{ pageSize: 8 }"
              ><template #columns
                ><a-table-column title="归档文件" data-index="name" /><a-table-column title="大小"
                  ><template #cell="{ record }">{{ bytes(record.bytes) }}</template></a-table-column
                ><a-table-column title="时间"
                  ><template #cell="{ record }">{{
                    date(record.createdAt)
                  }}</template></a-table-column
                ><a-table-column title="操作" :width="90"
                  ><template #cell="{ record }"
                    ><a
                      :href="`/api/v1/projects/${id}/backups/${encodeURIComponent(record.name)}`"
                      class="text-link"
                      >下载</a
                    ></template
                  ></a-table-column
                ></template
              ><template #empty><a-empty description="还没有备份归档" /></template></a-table
          ></a-tab-pane>
          <a-tab-pane key="storage" title="空间维护"
            ><div class="tab-intro">
              <div>
                <h3>保持工作空间轻盈</h3>
                <p class="muted">
                  可清理空间约 {{ bytes(reclaimable) }}，清理旧版本时会保护当前与上一版本。
                </p>
              </div>
              <a-button @click="load"><icon-refresh /></a-button>
            </div>
            <a-alert v-if="project.cleanupError" type="error" class="mb16">{{
              project.cleanupError
            }}</a-alert
            ><a-alert v-if="project.cleanup?.ActiveVersionUnknown" type="warning" class="mb16"
              >无法确定活跃版本，旧版本清理将被阻止。</a-alert
            >
            <div class="maintenance-row">
              <span class="maintenance-icon"><icon-file /></span>
              <div>
                <h4>Bundle 缓存</h4>
                <p>清除下载的压缩包，不影响已安装的版本。</p>
              </div>
              <strong>{{ bytes(project.cleanup?.BundleBytes) }}</strong
              ><a-button
                :disabled="project.busy"
                @click="act('clean', 'bundles', '将清除项目 Bundle 缓存；再次发布时需要重新下载。')"
                >清理缓存</a-button
              >
            </div>
            <div class="maintenance-row">
              <span class="maintenance-icon"><icon-history /></span>
              <div>
                <h4>历史版本</h4>
                <p>保留活跃版本、上一版本及 live unit 引用的版本。</p>
              </div>
              <strong>{{ bytes(project.cleanup?.OldReleaseBytes) }}</strong
              ><a-button
                :disabled="project.busy || project.cleanup?.ActiveVersionUnknown"
                @click="
                  act(
                    'clean',
                    'releases',
                    '只删除部署引擎判定为可清理的历史版本，删除后无法从本机回退到这些版本。',
                  )
                "
                >清理旧版本</a-button
              >
            </div>
            <div class="maintenance-row">
              <span class="maintenance-icon"><icon-file /></span>
              <div>
                <h4>残留临时目录</h4>
                <p>清除未完成发布留下的临时解压和暂存目录。</p>
              </div>
              <strong>{{ bytes(project.cleanup?.TmpBytes) }}</strong
              ><a-button
                :disabled="project.busy"
                @click="
                  act('clean', 'tmp', '将清除部署引擎识别出的残留临时目录，不影响已安装版本。')
                "
                >清理临时目录</a-button
              >
            </div>
            <div class="danger-zone">
              <div>
                <h4>移除项目</h4>
                <p>只移除登记，保留磁盘文件和任务记录。</p>
              </div>
              <a-button status="danger" :disabled="project.busy" @click="remove">移除登记</a-button>
            </div></a-tab-pane
          >
        </a-tabs>
      </section>
      <ProjectForm v-model:visible="edit" :project="project" @saved="load" /> </template
  ></a-spin>
  <TaskLog
    v-model:visible="logVisible"
    :task-id="logID"
    @update:visible="
      (value) => {
        if (!value) load()
      }
    "
  />
  <a-modal
    v-model:visible="deployVisible"
    title="发布新版本"
    :width="580"
    :footer="false"
    :mask-closable="!releasing"
    :closable="!releasing"
    ><p class="muted">先准备和校验产物，再激活新版本；激活或健康检查失败时尝试回滚。</p>
    <a-alert v-if="actionError" type="error" class="mb16">{{ actionError }}</a-alert
    ><a-radio-group v-model="source" type="button" class="mb20"
      ><a-radio value="release" :disabled="!project?.repository">GitHub Release</a-radio
      ><a-radio value="local">本地部署包</a-radio></a-radio-group
    ><a-form layout="vertical" :model="{ target }"
      ><template v-if="source === 'release'"
        ><a-form-item label="目标版本"
          ><a-input v-model="target" placeholder="latest 或 v1.0.0" /></a-form-item
        ><a-button :loading="releaseLoading" size="small" @click="loadReleases"
          >获取 Release 列表</a-button
        ><a-alert v-if="releaseError" type="warning" class="mt12">{{ releaseError }}</a-alert>
        <div v-if="releases.length" class="release-options">
          <button
            v-for="release in releases"
            :key="release.tag_name"
            :class="{ selected: target === release.tag_name }"
            @click="target = release.tag_name"
          >
            <span class="mono">{{ release.tag_name }}</span
            ><span>{{ release.name }}</span
            ><a-tag v-if="release.prerelease" size="small">预发布</a-tag>
          </button>
        </div></template
      ><template v-else
        ><input ref="selectedFile" type="file" accept=".zip,.enc" hidden @change="choose" /><button
          class="upload-drop"
          @click="selectedFile?.click()"
        >
          <icon-upload /><strong>{{ file?.name || '选择部署包' }}</strong
          ><span>{{ file ? bytes(file.size) : '支持 bundle.zip / bundle.zip.enc' }}</span>
        </button>
        <p class="muted small">压缩包根目录需包含 upy.yaml，name 与项目标识一致。</p></template
      ><a-checkbox v-model="force" class="mt20">强制重新部署相同版本</a-checkbox>
      <div class="modal-actions">
        <a-button :disabled="releasing" @click="deployVisible = false">取消</a-button
        ><a-button type="primary" :loading="releasing" @click="deploy">{{
          releasing && source === 'local' ? '正在上传…' : '开始发布'
        }}</a-button>
      </div></a-form
    ></a-modal
  >
</template>
