<template>
  <div class="test-route">
    <h1>测试路由页面</h1>
    <p>如果您能看到这个页面，说明路由系统正常工作。</p>
    <el-button @click="testFunction">测试按钮</el-button>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const message = ref('Hello World')

const testFunction = () => {
  console.log('测试函数被调用')
  message.value = '按钮被点击了'
}
</script>

<style scoped>
.test-route {
  padding: 20px;
}
</style>