<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '@/utils/api'

export default {
  name: 'App',
  setup() {
    const router = useRouter()

    const checkSystemStatus = async () => {
      try {
        // 检查系统是否需要初始化
        const response = await api.get('/setup/status')
        
        if (response.data.needs_setup) {
          // 如果需要初始化且当前不在引导页面，则跳转到引导页面
          if (router.currentRoute.value.path !== '/setup') {
            router.push('/setup')
          }
        }
      } catch (error) {
        console.error('检查系统状态失败:', error)
        // 如果检查失败，可能是网络问题，不强制跳转
      }
    }

    onMounted(() => {
      checkSystemStatus()
    })
  }
}
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  color: #2c3e50;
  height: 100vh;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  height: 100vh;
}
</style>