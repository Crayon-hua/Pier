import { reactive } from 'vue'
import { request, post } from '../api'

export const auth = reactive({ username: '', checked: false })
export async function checkAuth() {
  try {
    const user = await request<{ username: string }>('/auth/me')
    auth.username = user.username
  } catch {
    auth.username = ''
  } finally {
    auth.checked = true
  }
}
export async function logout() {
  await post('/auth/logout')
  auth.username = ''
}
