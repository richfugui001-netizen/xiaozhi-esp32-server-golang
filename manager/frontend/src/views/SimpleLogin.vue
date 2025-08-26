<template>
  <div style="padding: 50px;">
    <h1>简单登录测试</h1>
    <div style="max-width: 400px;">
      <div style="margin-bottom: 15px;">
        <label>用户名:</label>
        <input v-model="username" type="text" style="width: 100%; padding: 8px;" />
      </div>
      <div style="margin-bottom: 15px;">
        <label>密码:</label>
        <input v-model="password" type="password" style="width: 100%; padding: 8px;" />
      </div>
      <button @click="login" style="width: 100%; padding: 10px; background: #409EFF; color: white; border: none;">
        登录
      </button>
    </div>
    <div style="margin-top: 20px;">
      <p>调试信息:</p>
      <p>认证状态: {{ authStore.isAuthenticated }}</p>
      <p>用户信息: {{ JSON.stringify(authStore.user) }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const username = ref('admin')
const password = ref('password')

const login = async () => {
  try {
    const result = await authStore.login({
      username: username.value,
      password: password.value
    })
    
    if (result.success) {
      alert('登录成功!')
      if (authStore.user?.role === 'admin') {
        router.push('/dashboard')
      } else {
        router.push('/agents')
      }
    } else {
      alert('登录失败: ' + result.message)
    }
  } catch (error) {
    alert('登录错误: ' + error.message)
  }
}
</script>