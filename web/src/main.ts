import { createApp } from 'vue'
import ArcoVue from '@arco-design/web-vue'
import {
  IconApps,
  IconFolder,
  IconHistory,
  IconSettings,
  IconRight,
  IconExport,
  IconArrowRight,
  IconUser,
  IconSafe,
  IconPlus,
  IconCheckCircle,
  IconSync,
  IconStorage,
  IconFolderAdd,
  IconCodeSquare,
  IconCommon,
  IconDesktop,
  IconInfoCircle,
  IconRefresh,
  IconGithub,
  IconArrowLeft,
  IconSend,
  IconSave,
  IconFile,
  IconUpload,
  IconDownload,
} from '@arco-design/web-vue/es/icon'
import '@arco-design/web-vue/dist/arco.css'
import App from './App.vue'
import router from './router'
import './style.css'

const app = createApp(App).use(ArcoVue).use(router)
const icons = {
  IconApps,
  IconFolder,
  IconHistory,
  IconSettings,
  IconRight,
  IconExport,
  IconArrowRight,
  IconUser,
  IconSafe,
  IconPlus,
  IconCheckCircle,
  IconSync,
  IconStorage,
  IconFolderAdd,
  IconCodeSquare,
  IconCommon,
  IconDesktop,
  IconInfoCircle,
  IconRefresh,
  IconGithub,
  IconArrowLeft,
  IconSend,
  IconSave,
  IconFile,
  IconUpload,
  IconDownload,
}
for (const [name, component] of Object.entries(icons)) app.component(name, component)
app.mount('#app')
