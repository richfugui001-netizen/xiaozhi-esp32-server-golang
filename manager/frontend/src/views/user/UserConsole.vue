<template>
  <div class="user-console">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon">
            <Monitor />
          </el-icon>
          <h1 class="page-title">智能体控制台</h1>
        </div>
        <p class="page-description">管理您的设备和智能体，实时监控运行状态</p>
      </div>
    </div>

    <!-- 设备状态卡片 -->
    <div class="devices-section">
      <h2 class="section-title">
        <el-icon><Connection /></el-icon>
        我的设备
      </h2>
      
      <div v-if="devices.length === 0" class="empty-state">
        <el-empty description="暂无设备">
          <el-button type="primary" @click="showAddDevice = true">
            <el-icon><Plus /></el-icon>
            添加设备
          </el-button>
        </el-empty>
      </div>
      
      <div v-else class="devices-grid">
        <el-row :gutter="20">
          <el-col :span="8" v-for="device in devices" :key="device.id">
            <el-card class="device-card" shadow="hover">
              <template #header>
                <div class="device-header">
                  <div class="device-info">
                    <h3 class="device-name">{{ device.name }}</h3>
                    <el-tag 
                      :type="device.status === 'online' ? 'success' : 'danger'"
                      size="small"
                    >
                      {{ device.status === 'online' ? '在线' : '离线' }}
                    </el-tag>
                  </div>
                  <div class="device-actions">
                    <el-button 
                      type="primary" 
                      size="small" 
                      @click="openDeviceControl(device)"
                    >
                      控制
                    </el-button>
                  </div>
                </div>
              </template>
              
              <div class="device-content">
                <div class="device-stats">
                  <div class="stat-item">
                    <span class="stat-label">语音识别:</span>
                    <el-tag :type="device.vad_status ? 'success' : 'info'" size="small">
                      {{ device.vad_status ? '启用' : '禁用' }}
                    </el-tag>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">智能体:</span>
                    <span class="stat-value">{{ device.agent_name || '未绑定' }}</span>
                  </div>
                  <div class="stat-item">
                    <span class="stat-label">最后活跃:</span>
                    <span class="stat-value">{{ formatTime(device.last_active) }}</span>
                  </div>
                </div>
                
                <!-- 语音识别控制 -->
                <div class="voice-control">
                  <div class="control-header">
                    <el-icon><Microphone /></el-icon>
                    <span>语音识别</span>
                  </div>
                  <div class="control-actions">
                    <el-button 
                      :type="device.vad_status ? 'danger' : 'success'"
                      size="small"
                      @click="toggleVAD(device)"
                      :loading="device.loading"
                    >
                      {{ device.vad_status ? '停止识别' : '开始识别' }}
                    </el-button>
                  </div>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
      
      <!-- 查看更多按钮 -->
      <div v-if="allDevicesData.length > 6 && !showAllDevices" class="show-more-section">
        <el-button type="text" @click="toggleShowAllDevices">
          查看更多设备 ({{ allDevicesData.length - 6 }})
          <el-icon><ArrowDown /></el-icon>
        </el-button>
      </div>
      
      <div v-if="showAllDevices && allDevicesData.length > 6" class="show-less-section">
        <el-button type="text" @click="toggleShowAllDevices">
          收起设备列表
          <el-icon><ArrowUp /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- 智能体快速访问 -->
    <div class="agents-section">
      <div class="section-header">
        <h2 class="section-title">
          <el-icon><Monitor /></el-icon>
          我的智能体
        </h2>
        <el-button type="primary" @click="$router.push('/agents')">
          <el-icon><Setting /></el-icon>
          管理智能体
        </el-button>
      </div>
      
      <div v-if="agents.length === 0" class="empty-state">
        <el-empty description="暂无智能体">
          <el-button type="primary" @click="$router.push('/agents')">
            <el-icon><Plus /></el-icon>
            创建智能体
          </el-button>
        </el-empty>
      </div>
      
      <div v-else class="agents-grid">
        <el-row :gutter="20">
          <el-col :span="6" v-for="agent in agents.slice(0, 4)" :key="agent.id">
            <el-card class="agent-card" shadow="hover" @click="selectAgent(agent)">
              <div class="agent-content">
                <div class="agent-icon">
                  <el-icon size="32"><Monitor /></el-icon>
                </div>
                <div class="agent-info">
                  <h4 class="agent-name">{{ agent.name }}</h4>
                  <p class="agent-desc">{{ agent.description || '暂无描述' }}</p>
                  <el-tag 
                    :type="agent.status === 'active' ? 'success' : 'info'"
                    size="small"
                  >
                    {{ agent.status === 'active' ? '活跃' : '非活跃' }}
                  </el-tag>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </div>

    <!-- 添加设备弹窗 -->
    <el-dialog
      v-model="showAddDevice"
      title="添加设备"
      width="500px"
    >
      <el-form
        ref="deviceFormRef"
        :model="deviceForm"
        :rules="deviceRules"
        label-width="100px"
      >
        <el-form-item label="设备名称" prop="name">
          <el-input
            v-model="deviceForm.name"
            placeholder="请输入设备名称"
          />
        </el-form-item>
        <el-form-item label="设备激活码" prop="device_code">
          <el-input
            v-model="deviceForm.device_code"
            placeholder="请输入设备激活码"
          />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="deviceForm.description"
            type="textarea"
            placeholder="请输入设备描述（可选）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="showAddDevice = false">取消</el-button>
          <el-button type="primary" @click="addDevice" :loading="adding">
            添加
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 设备控制弹窗 -->
    <el-dialog
      v-model="showDeviceControl"
      :title="`控制设备: ${currentDevice?.name}`"
      width="600px"
    >
      <div v-if="currentDevice" class="device-control-panel">
        <div class="control-section">
          <h4>基础控制</h4>
          <div class="control-buttons">
            <el-button type="success" @click="sendCommand('wake_up')">
              <el-icon><VideoPlay /></el-icon>
              唤醒设备
            </el-button>
            <el-button type="warning" @click="sendCommand('sleep')">
              <el-icon><VideoPause /></el-icon>
              休眠设备
            </el-button>
            <el-button type="info" @click="sendCommand('restart')">
              <el-icon><Refresh /></el-icon>
              重启设备
            </el-button>
          </div>
        </div>
        
        <div class="control-section">
          <h4>语音控制</h4>
          <div class="voice-settings">
            <el-form label-width="100px">
              <el-form-item label="音量">
                <el-slider v-model="currentDevice.volume" :max="100" />
              </el-form-item>
              <el-form-item label="语音识别">
                <el-switch 
                  v-model="currentDevice.vad_status"
                  @change="toggleVAD(currentDevice)"
                />
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Monitor,
  Connection,
  Plus,
  Setting,
  Microphone,
  VideoPlay,
  VideoPause,
  Refresh,
  ArrowDown,
  ArrowUp
} from '@element-plus/icons-vue'
import api from '../../utils/api'

const devices = ref([])
const agents = ref([])
const allDevicesData = ref([])
const showAddDevice = ref(false)
const showDeviceControl = ref(false)
const currentDevice = ref(null)
const adding = ref(false)
const deviceFormRef = ref(null)
const showAllDevices = ref(false)

const deviceForm = reactive({
  name: '',
  device_code: '',
  description: ''
})

const deviceRules = {
  name: [{ required: true, message: '请输入设备名称', trigger: 'blur' }],
  device_code: [{ required: true, message: '请输入设备激活码', trigger: 'blur' }]
}

// 加载设备列表
const loadDevices = async () => {
  try {
    const response = await api.get('/user/devices')
    const allDevices = response.data.data || []
    // 保存所有设备数据
    allDevicesData.value = allDevices.map(device => ({
      ...device,
      loading: false,
      volume: device.volume || 80
    }))
    // 限制显示最多6个设备
    devices.value = showAllDevices.value ? allDevicesData.value : allDevicesData.value.slice(0, 6)
  } catch (error) {
    console.error('加载设备失败:', error)
    ElMessage.error('加载设备失败')
    devices.value = []
    allDevicesData.value = []
  }
}

// 加载智能体列表
const loadAgents = async () => {
  try {
    const response = await api.get('/user/agents')
    agents.value = response.data.data || []
  } catch (error) {
    console.error('加载智能体失败:', error)
    ElMessage.error('加载智能体失败')
    agents.value = []
  }
}

// 切换语音识别状态
const toggleVAD = async (device) => {
  device.loading = true
  try {
    // 模拟API调用
    await new Promise(resolve => setTimeout(resolve, 1000))
    device.vad_status = !device.vad_status
    ElMessage.success(`${device.vad_status ? '启用' : '禁用'}语音识别成功`)
  } catch (error) {
    console.error('切换语音识别失败:', error)
    ElMessage.error('操作失败')
  } finally {
    device.loading = false
  }
}

// 打开设备控制面板
const openDeviceControl = (device) => {
  currentDevice.value = device
  showDeviceControl.value = true
}

// 发送设备命令
const sendCommand = async (command) => {
  try {
    // 模拟API调用
    await new Promise(resolve => setTimeout(resolve, 500))
    ElMessage.success(`命令 ${command} 发送成功`)
  } catch (error) {
    console.error('发送命令失败:', error)
    ElMessage.error('发送命令失败')
  }
}

// 选择智能体
const selectAgent = (agent) => {
  ElMessage.info(`选择了智能体: ${agent.name}`)
  // 可以跳转到智能体详情页或执行其他操作
}

// 添加设备
const addDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
  } catch (error) {
    return
  }
  
  adding.value = true
  try {
    // 模拟API调用
    await new Promise(resolve => setTimeout(resolve, 1000))
    ElMessage.success('添加设备成功')
    showAddDevice.value = false
    // 重置表单
    Object.assign(deviceForm, {
      name: '',
      device_code: '',
      description: ''
    })
    // 重新加载设备列表
    await loadDevices()
  } catch (error) {
    console.error('添加设备失败:', error)
    ElMessage.error('添加设备失败')
  } finally {
    adding.value = false
  }
}

// 切换显示所有设备
const toggleShowAllDevices = () => {
  showAllDevices.value = !showAllDevices.value
  devices.value = showAllDevices.value ? allDevicesData.value : allDevicesData.value.slice(0, 6)
}

// 格式化时间
const formatTime = (date) => {
  if (!date) return '未知'
  const now = new Date()
  const diff = now - new Date(date)
  const minutes = Math.floor(diff / (1000 * 60))
  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days < 30) return `${days}天前`
  return `${Math.floor(days / 30)}个月前`
}

onMounted(() => {
  loadDevices()
  loadAgents()
})
</script>

<style scoped>
.user-console {
  min-height: 100vh;
  background: #f8f9fa;
  padding: 24px;
}

/* 页面头部 */
.page-header {
  margin-bottom: 32px;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
}

.title-section {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 8px;
}

.title-icon {
  font-size: 32px;
  color: #409eff;
}

.page-title {
  font-size: 28px;
  font-weight: 600;
  color: #1f2937;
  margin: 0;
  background: linear-gradient(135deg, #409eff 0%, #67c23a 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.page-description {
  font-size: 16px;
  color: #6b7280;
  margin: 0;
  margin-left: 48px;
}

/* 区域标题 */
.section-title {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 20px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 20px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

/* 设备区域 */
.devices-section {
  max-width: 1200px;
  margin: 0 auto 40px;
}

.devices-grid {
  margin-bottom: 20px;
}

.device-card {
  border-radius: 12px;
  transition: all 0.3s ease;
  border: 1px solid #e5e7eb;
}

.device-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 25px -3px rgba(0, 0, 0, 0.1);
}

.device-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.device-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.device-name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.device-content {
  padding: 16px 0;
}

.device-stats {
  margin-bottom: 16px;
}

.stat-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.stat-label {
  font-size: 14px;
  color: #6b7280;
}

.stat-value {
  font-size: 14px;
  color: #1f2937;
  font-weight: 500;
}

.voice-control {
  border-top: 1px solid #e5e7eb;
  padding-top: 16px;
}

.control-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.control-actions {
  display: flex;
  gap: 8px;
}

/* 智能体区域 */
.agents-section {
  max-width: 1200px;
  margin: 0 auto;
}

.agents-grid {
  margin-bottom: 20px;
}

.agent-card {
  border-radius: 12px;
  transition: all 0.3s ease;
  cursor: pointer;
  border: 1px solid #e5e7eb;
}

.agent-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 25px -3px rgba(0, 0, 0, 0.1);
  border-color: #409eff;
}

.agent-content {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px;
}

.agent-icon {
  color: #409eff;
}

.agent-info {
  flex: 1;
}

.agent-name {
  margin: 0 0 8px 0;
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.agent-desc {
  margin: 0 0 8px 0;
  font-size: 14px;
  color: #6b7280;
  line-height: 1.4;
}

/* 空状态 */
.empty-state {
  text-align: center;
  padding: 40px 20px;
}

/* 查看更多按钮 */
.show-more-section,
.show-less-section {
  text-align: center;
  margin-top: 20px;
  padding: 16px;
}

.show-more-section .el-button,
.show-less-section .el-button {
  font-size: 14px;
  color: #409eff;
}

.show-more-section .el-button:hover,
.show-less-section .el-button:hover {
  color: #66b1ff;
}

/* 设备控制面板 */
.device-control-panel {
  padding: 20px 0;
}

.control-section {
  margin-bottom: 24px;
}

.control-section h4 {
  margin: 0 0 16px 0;
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.control-buttons {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.voice-settings {
  background: #f8f9fa;
  padding: 16px;
  border-radius: 8px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .user-console {
    padding: 16px;
  }
  
  .page-title {
    font-size: 24px;
  }
  
  .devices-grid .el-col,
  .agents-grid .el-col {
    margin-bottom: 16px;
  }
  
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
}

@media (max-width: 480px) {
  .title-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .page-description {
    margin-left: 0;
  }
  
  .device-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  
  .control-buttons {
    flex-direction: column;
  }
}
</style>