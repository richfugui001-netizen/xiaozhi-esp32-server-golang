<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon">
              <el-icon size="40" color="#409EFF"><User /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-number">{{ stats.totalUsers }}</div>
              <div class="stat-label">总用户数</div>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon">
              <el-icon size="40" color="#67C23A"><Monitor /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-number">{{ stats.totalDevices }}</div>
              <div class="stat-label">设备总数</div>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon">
              <el-icon size="40" color="#E6A23C"><Robot /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-number">{{ stats.totalAgents }}</div>
              <div class="stat-label">智能体数量</div>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon">
              <el-icon size="40" color="#F56C6C"><Connection /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-number">{{ stats.onlineDevices }}</div>
              <div class="stat-label">在线设备</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
    
    <el-row :gutter="20" style="margin-top: 20px;">
      <el-col :span="12">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>系统信息</span>
            </div>
          </template>
          <div class="system-info">
            <div class="info-item">
              <span class="info-label">系统版本：</span>
              <span class="info-value">v1.0.0</span>
            </div>
            <div class="info-item">
              <span class="info-label">运行时间：</span>
              <span class="info-value">{{ uptime }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">当前用户：</span>
              <span class="info-value">{{ authStore.user?.username }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">用户角色：</span>
              <el-tag :type="authStore.isAdmin ? 'danger' : 'primary'">
                {{ authStore.isAdmin ? '管理员' : '普通用户' }}
              </el-tag>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="12">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>快速操作</span>
            </div>
          </template>
          <div class="quick-actions">
            <template v-if="authStore.isAdmin">
              <el-button type="primary" @click="$router.push('/admin/users')">
                <el-icon><User /></el-icon>
                用户管理
              </el-button>
              <el-button type="success" @click="$router.push('/admin/llm-config')">
                <el-icon><Setting /></el-icon>
                LLM配置
              </el-button>
              <el-button type="warning" @click="$router.push('/admin/vad-config')">
                <el-icon><Setting /></el-icon>
                VAD配置
              </el-button>
            </template>
            <template v-else>
              <el-button type="primary" @click="$router.push('/agents')">
            <el-icon><Monitor /></el-icon>
            智能体管理
          </el-button>
              <el-text type="info">
                普通用户主要功能在智能体管理页面
              </el-text>
            </template>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import {
  User,
  Monitor,
  Connection,
  Setting,
  Plus
} from '@element-plus/icons-vue'

const authStore = useAuthStore()

const stats = ref({
  totalUsers: 0,
  totalDevices: 0,
  totalAgents: 0,
  onlineDevices: 0
})

const uptime = ref('0天 0小时 0分钟')

onMounted(() => {
  // 模拟数据，实际应该从API获取
  stats.value = {
    totalUsers: 156,
    totalDevices: 89,
    totalAgents: 234,
    onlineDevices: 67
  }
  
  // 模拟运行时间
  const startTime = new Date('2024-01-01')
  const now = new Date()
  const diff = now - startTime
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
  uptime.value = `${days}天 ${hours}小时 ${minutes}分钟`
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.stat-card {
  height: 120px;
}

.stat-content {
  display: flex;
  align-items: center;
  height: 100%;
}

.stat-icon {
  margin-right: 20px;
}

.stat-info {
  flex: 1;
}

.stat-number {
  font-size: 32px;
  font-weight: bold;
  color: #333;
  line-height: 1;
}

.stat-label {
  font-size: 14px;
  color: #666;
  margin-top: 8px;
}

.card-header {
  font-weight: bold;
  font-size: 16px;
}

.system-info {
  padding: 10px 0;
}

.info-item {
  display: flex;
  align-items: center;
  margin-bottom: 15px;
}

.info-item:last-child {
  margin-bottom: 0;
}

.info-label {
  width: 100px;
  color: #666;
}

.info-value {
  color: #333;
  font-weight: 500;
}

.quick-actions {
  display: flex;
  flex-direction: column;
  gap: 15px;
  padding: 10px 0;
}

.quick-actions .el-button {
  justify-content: flex-start;
}
</style>