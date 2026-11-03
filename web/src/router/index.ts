import { createRouter, createWebHistory } from 'vue-router'
import { auth, checkAuth } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      component: () => import('../views/Login.vue'),
      meta: { title: '登录', public: true },
    },
    { path: '/', component: () => import('../views/Overview.vue'), meta: { title: '概览' } },
    {
      path: '/projects',
      component: () => import('../views/Projects.vue'),
      meta: { title: '项目' },
    },
    {
      path: '/projects/:id',
      component: () => import('../views/ProjectDetail.vue'),
      meta: { title: '项目详情' },
    },
    { path: '/tasks', component: () => import('../views/Tasks.vue'), meta: { title: '任务记录' } },
    {
      path: '/settings',
      component: () => import('../views/Settings.vue'),
      meta: { title: '设置' },
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
router.beforeEach(async (to) => {
  if (!auth.checked) await checkAuth()
  if (!to.meta.public && !auth.username) return '/login'
  if (to.path === '/login' && auth.username) return '/'
})
router.afterEach((to) => {
  document.title = `${String(to.meta.title || '')} · Pier 栈桥`
})
window.addEventListener('pier:unauthorized', () => {
  auth.username = ''
  void router.replace('/login')
})
export default router
