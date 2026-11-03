<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { message, request, type Project } from '../api'
const props = defineProps<{ visible: boolean; project?: Project }>()
const emit = defineEmits<{ 'update:visible': [value: boolean]; saved: [] }>()
const saving = ref(false),
  error = ref('')
const form = reactive({
  id: '',
  name: '',
  description: '',
  type: 'static',
  repository: '',
  asset: 'bundle.zip',
  githubToken: '',
  clearToken: false,
  configYaml: 'bundles:\n  keep: 3\n',
})
watch(
  () => props.visible,
  (value) => {
    if (value) {
      error.value = ''
      Object.assign(
        form,
        {
          id: '',
          name: '',
          description: '',
          type: 'static',
          repository: '',
          asset: 'bundle.zip',
          configYaml: 'bundles:\n  keep: 3\n',
        },
        props.project || {},
        { githubToken: '', clearToken: false },
      )
    }
  },
)
async function save() {
  saving.value = true
  error.value = ''
  try {
    const payload = {
      id: form.id,
      name: form.name,
      description: form.description,
      type: form.type,
      repository: form.repository,
      asset: form.asset,
      githubToken: form.githubToken,
      clearToken: form.clearToken,
      configYaml: form.configYaml,
    }
    await request(`/projects${props.project ? `/${props.project.id}` : ''}`, {
      method: props.project ? 'PUT' : 'POST',
      body: JSON.stringify(payload),
    })
    Message.success(props.project ? '项目已更新' : '项目已创建')
    emit('saved')
    emit('update:visible', false)
  } catch (e) {
    error.value = message(e)
  } finally {
    saving.value = false
  }
}
</script>
<template>
  <a-drawer
    :visible="visible"
    :width="560"
    :title="project ? '项目设置' : '新建项目'"
    :ok-loading="saving"
    unmount-on-close
    @cancel="emit('update:visible', false)"
    @ok="save"
    ><p class="muted">登记一个应用，部署产物和持久化数据将保存在独立项目目录。</p>
    <a-alert v-if="error" type="error" class="mb16">{{ error }}</a-alert
    ><a-form :model="form" layout="vertical"
      ><a-form-item label="项目名称" required
        ><a-input v-model="form.name" placeholder="例如：公司官网" :max-length="100" /></a-form-item
      ><a-form-item
        label="项目标识"
        required
        extra="2–48 位小写字母、数字和连字符；须与包内 upy.yaml 的 name 一致。"
        ><a-input v-model="form.id" :disabled="!!project" placeholder="company-web" /></a-form-item
      ><a-form-item label="部署类型"
        ><a-radio-group v-model="form.type" type="button"
          ><a-radio value="static">静态站点</a-radio><a-radio value="docker">Docker</a-radio
          ><a-radio value="system">系统服务</a-radio></a-radio-group
        ></a-form-item
      ><a-form-item label="描述"
        ><a-textarea
          v-model="form.description"
          placeholder="这个项目用来做什么？"
          :max-length="300" /></a-form-item
      ><a-divider>发布来源</a-divider
      ><a-form-item label="GitHub 仓库" extra="可留空，使用本地部署包上传。"
        ><a-input v-model="form.repository" placeholder="owner/repository" /></a-form-item
      ><a-form-item label="Release 资产名称"
        ><a-input v-model="form.asset" placeholder="bundle.zip" /></a-form-item
      ><a-form-item
        label="GitHub Token"
        :extra="
          project?.hasToken
            ? '已配置，留空保留原 Token。'
            : '公开仓库可留空；私有仓库需要读取 Release 的权限。'
        "
        ><a-input-password
          v-model="form.githubToken"
          autocomplete="new-password"
          placeholder="输入 Token" /></a-form-item
      ><a-checkbox v-if="project?.hasToken" v-model="form.clearToken">清除已有 Token</a-checkbox
      ><a-divider>高级配置</a-divider>
      <p class="muted small">
        支持 upy 的 service、crypto、backup、notify 配置；name 与 release
        由上方表单管理。此处可能含密钥，仅管理员可见。
      </p>
      <a-form-item label="项目 YAML"
        ><a-textarea
          v-model="form.configYaml"
          class="code-input"
          :auto-size="{ minRows: 8, maxRows: 22 }"
          spellcheck="false"
      /></a-form-item>
      <details class="config-example">
        <summary>查看配置示例</summary>
        <pre>
service:
  user: www-data
  group: www-data
crypto:
  password: your-bundle-password
backup:
  script: backup.sh
  retention: 7
  timeout: 15m
notify:
  bark:
    - your-device-key</pre
        >
      </details></a-form
    ></a-drawer
  >
</template>
