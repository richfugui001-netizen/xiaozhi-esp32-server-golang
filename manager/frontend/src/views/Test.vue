<template>
  <div class="test-page">
    <h1>测试页面</h1>
    <p>如果你能看到这个页面，说明路由系统正常工作。</p>
    <div class="debug-info">
      <h3>调试信息：</h3>
      <p>当前路径: {{ $route.path }}</p>
      <p>用户认证状态: {{ authStore.isAuthenticated }}</p>
      <p>用户信息: {{ authStore.user }}</p>
      <p>是否管理员: {{ authStore.isAdmin }}</p>
    </div>
    <div class="actions">
      <el-button @click="$router.push('/login')">去登录页</el-button>
      <el-button @click="$router.push('/dashboard')">去仪表板</el-button>
      <el-button @click="$router.push('/agents')">去智能体列表</el-button>
    </div>
  </div>
</template>

<script setup>
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
</script>

<style scoped>
.test-page {
  padding: 20px;
}

.debug-info {
  background: #f5f5f5;
  padding: 15px;
  border-radius: 4px;
  margin: 20px 0;
}

.actions {
  margin-top: 20px;
}

.actions .el-button {
  margin-right: 10px;
}
</style>