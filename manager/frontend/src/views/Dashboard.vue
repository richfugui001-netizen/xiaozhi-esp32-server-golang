<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="6" v-if="authStore.isAdmin">
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
      
      <el-col :span="authStore.isAdmin ? 6 : 8">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon">
              <el-icon size="40" color="#67C23A"><Monitor /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-number">{{ stats.totalDevices }}</div>
              <div class="stat-label">{{ authStore.isAdmin ? '设备总数' : '我的设备' }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="authStore.isAdmin ? 6 : 8">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon">
              <el-icon size="40" color="#E6A23C"><Cpu /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-number">{{ stats.totalAgents }}</div>
              <div class="stat-label">{{ authStore.isAdmin ? '智能体数量' : '我的智能体' }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="authStore.isAdmin ? 6 : 8">
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
    
    <!-- 配置管理卡片 - 放在统计数据和系统信息之间 -->
    <el-card class="config-card" v-if="authStore.isAdmin" style="margin: 20px 0;">
      <template #header>
        <div class="config-header">
          <el-icon size="18" color="#409EFF"><Setting /></el-icon>
          <span>配置管理</span>
        </div>
      </template>
      <div class="config-actions">
        <el-button 
          type="primary" 
          @click="exportConfig"
          class="config-btn"
        >
          <el-icon><Download /></el-icon>
          导出配置
        </el-button>
                 <el-button 
           type="success" 
           @click="importConfig"
           class="config-btn"
         >
           <el-icon><Upload /></el-icon>
           导入配置
           <div class="btn-tip">支持YAML/JSON</div>
         </el-button>
      </div>
      <input
        ref="fileInput"
        type="file"
        accept=".yaml,.yml,.json"
        style="display: none"
        @change="handleFileChange"
      />
    </el-card>
    
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
import { useAuthStore } from '@/stores/auth'
import api from '@/utils/api'
import { ElMessage } from 'element-plus'
import {
  User,
  Monitor,
  Connection,
  Setting,
  Plus,
  Download,
  Upload,
  Cpu
} from '@element-plus/icons-vue'

const authStore = useAuthStore()

const stats = ref({
  totalUsers: 0,
  totalDevices: 0,
  totalAgents: 0,
  onlineDevices: 0
})

const uptime = ref('0天 0小时 0分钟')
const fileInput = ref(null)

onMounted(async () => {
  await loadStats()
  
  // 模拟运行时间
  const startTime = new Date('2024-01-01')
  const now = new Date()
  const diff = now - startTime
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
  uptime.value = `${days}天 ${hours}小时 ${minutes}分钟`
})

// 加载统计数据
const loadStats = async () => {
  try {
    const response = await api.get('/dashboard/stats')
    stats.value = {
      totalUsers: response.data.totalUsers || 0,
      totalDevices: response.data.totalDevices || 0,
      totalAgents: response.data.totalAgents || 0,
      onlineDevices: response.data.onlineDevices || 0
    }
  } catch (error) {
    console.error('加载统计数据失败:', error)
    // 使用默认值
    stats.value = {
      totalUsers: 0,
      totalDevices: 0,
      totalAgents: 0,
      onlineDevices: 0
    }
  }
}

// 导出配置
const exportConfig = async () => {
  try {
    const response = await fetch('/api/admin/configs/export', {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${authStore.token}`
      }
    })
    
    if (response.ok) {
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'config.yaml'
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
      
      ElMessage.success('配置导出成功')
    } else {
      ElMessage.error('配置导出失败')
    }
  } catch (error) {
    console.error('导出配置失败:', error)
    ElMessage.error('配置导出失败')
  }
}

// 导入配置
const importConfig = () => {
  fileInput.value.click()
}

// 处理文件选择
const handleFileChange = async (event) => {
  const file = event.target.files[0]
  if (!file) return
  
  // 检查文件格式
  const validExtensions = ['.yaml', '.yml', '.json']
  const fileExtension = file.name.toLowerCase().substring(file.name.lastIndexOf('.'))
  
  if (!validExtensions.includes(fileExtension)) {
    ElMessage.error('请选择YAML或JSON格式的文件')
    return
  }
  
  const formData = new FormData()
  formData.append('file', file)
  
  try {
    const response = await fetch('/api/admin/configs/import', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${authStore.token}`
      },
      body: formData
    })
    
    if (response.ok) {
      ElMessage.success('配置导入成功')
    } else {
      const error = await response.json()
      ElMessage.error(error.error || '配置导入失败')
    }
  } catch (error) {
    console.error('导入配置失败:', error)
    ElMessage.error('配置导入失败')
  }
  
  // 清空文件输入
  event.target.value = ''
}
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.config-card {
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.config-header {
  display: flex;
  align-items: center;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.config-header .el-icon {
  margin-right: 8px;
}

.config-actions {
  display: flex;
  gap: 15px;
  padding: 10px 0;
}

.config-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px 20px;
  border-radius: 6px;
  transition: all 0.3s ease;
  font-weight: 500;
}

.config-btn .el-icon {
  margin-right: 8px;
  font-size: 16px;
}

.config-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.config-btn {
  position: relative;
}

.btn-tip {
  position: absolute;
  bottom: -20px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 10px;
  color: #909399;
  white-space: nowrap;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.config-btn:hover .btn-tip {
  opacity: 1;
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