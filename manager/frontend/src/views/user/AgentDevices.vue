<template>
  <div class="agent-devices-page">
    <div class="page-header">
      <div class="header-left">
        <el-button @click="goBack" type="text" class="back-btn">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <div class="header-info">
          <h2>设备管理</h2>
          <p class="page-subtitle">管理智能体关联的设备</p>
        </div>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showAddDeviceDialog = true">
          <el-icon><Plus /></el-icon>
          添加设备
        </el-button>
      </div>
    </div>

    <div v-if="devices.length === 0" class="empty-section">
      <el-card class="empty-card">
        <div class="empty-content">
          <el-icon size="64" color="#909399"><Monitor /></el-icon>
          <h3>暂无设备</h3>
          <p>该智能体还没有关联任何设备。</p>
          <div class="empty-actions">
            <el-button type="primary" size="large" @click="showAddDeviceDialog = true">
              <el-icon><Plus /></el-icon>
              添加第一个设备
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else class="devices-grid">
      <el-row :gutter="24">
        <el-col :xs="24" :sm="12" :md="8" :lg="6" v-for="device in devices" :key="device.id">
          <div class="device-card">
            <div class="device-header">
              <div class="device-icon">
                <el-icon size="28"><Monitor /></el-icon>
              </div>
              <div class="device-info">
                <h3 class="device-name">{{ device.name }}</h3>
                <p class="device-code">{{ device.code }}</p>
              </div>
              <div class="device-status">
                <span :class="['status-dot', device.online ? 'online' : 'offline']"></span>
                <span class="status-text">{{ device.online ? '在线' : '离线' }}</span>
              </div>
            </div>
            
            <div class="device-meta">
              <div class="meta-row">
                <span class="meta-label">设备类型</span>
                <span class="meta-value">ESP32设备</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">最后活跃</span>
                <span class="meta-value">{{ formatDate(device.last_seen) }}</span>
              </div>
            </div>
            
            <div class="device-actions">
              <el-button size="small" @click="handleDeviceConfig(device.id)">
                <el-icon><Setting /></el-icon>
                配置
              </el-button>
              <el-button size="small" type="danger" @click="handleRemoveDevice(device.id)">
                <el-icon><Delete /></el-icon>
                移除
              </el-button>
            </div>
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 添加设备弹窗 -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="添加设备"
      width="400px"
      :before-close="handleCloseAddDevice"
    >
      <div class="device-dialog-content">
        <div class="device-icon">
          <el-icon size="48"><Monitor /></el-icon>
        </div>
        <p class="device-tip">请输入设备验证码</p>
        <el-form
          ref="deviceFormRef"
          :model="deviceForm"
          :rules="deviceRules"
        >
          <el-form-item prop="code">
            <el-input
              v-model="deviceForm.code"
              placeholder="请输入6位验证码"
              size="large"
              :maxlength="6"
              style="text-align: center; font-size: 18px; letter-spacing: 4px;"
            />
          </el-form-item>
        </el-form>
      </div>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseAddDevice" size="large">取消</el-button>
          <el-button type="primary" @click="handleAddDevice" :loading="addingDevice" size="large">
            确定
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Plus, Monitor, Setting, Delete } from '@element-plus/icons-vue'
import api from '../../utils/api'

const router = useRouter()
const route = useRoute()

const agentId = route.params.id
const devices = ref([])
const showAddDeviceDialog = ref(false)
const addingDevice = ref(false)
const deviceFormRef = ref()

const deviceForm = reactive({
  code: ''
})

const deviceRules = {
  code: [
    { required: true, message: '请输入设备验证码', trigger: 'blur' },
    { len: 6, message: '验证码长度为6位', trigger: 'blur' }
  ]
}

const loadDevices = async () => {
  try {
    const response = await api.get(`/user/agents/${agentId}/devices`)
    devices.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载设备列表失败')
  }
}

const handleAddDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
    addingDevice.value = true
    
    const response = await api.post(`/user/agents/${agentId}/devices`, {
      code: deviceForm.code
    })
    
    if (response.data.success) {
      ElMessage.success('设备添加成功')
      handleCloseAddDevice()
      await loadDevices()
    }
  } catch (error) {
    console.error('添加设备失败:', error)
    ElMessage.error('添加设备失败')
  } finally {
    addingDevice.value = false
  }
}

const handleCloseAddDevice = () => {
  showAddDeviceDialog.value = false
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
  Object.assign(deviceForm, { code: '' })
}

const handleDeviceConfig = (deviceId) => {
  ElMessage.info('设备配置功能开发中')
}

const handleRemoveDevice = async (deviceId) => {
  try {
    await ElMessageBox.confirm(
      '确定要移除这个设备吗？',
      '确认移除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    const response = await api.delete(`/user/agents/${agentId}/devices/${deviceId}`)
    if (response.data.success) {
      ElMessage.success('设备移除成功')
      await loadDevices()
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('移除设备失败')
    }
  }
}

const goBack = () => {
  router.push('/agents')
}

const formatDate = (dateString) => {
  if (!dateString) return '从未'
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadDevices()
})
</script>

<style scoped>
.agent-devices-page {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 20px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 15px;
}

.back-btn {
  padding: 8px;
  color: #409EFF;
}

.header-info h2 {
  margin: 0;
  color: #333;
}

.page-subtitle {
  margin: 5px 0 0 0;
  color: #666;
  font-size: 14px;
}

.empty-section {
  margin-top: 40px;
}

.empty-card {
  text-align: center;
  padding: 40px 20px;
}

.empty-content h3 {
  margin: 20px 0 10px 0;
  color: #333;
}

.empty-content p {
  color: #666;
  margin-bottom: 30px;
}

.devices-grid {
  margin-top: 20px;
}

.device-card {
  background: white;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.device-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}

.device-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}

.device-icon {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  flex-shrink: 0;
}

.device-info {
  flex: 1;
  min-width: 0;
}

.device-name {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.device-code {
  margin: 0;
  font-size: 12px;
  color: #999;
  font-family: monospace;
}

.device-status {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ddd;
}

.status-dot.online {
  background: #67c23a;
}

.status-dot.offline {
  background: #f56c6c;
}

.status-text {
  font-size: 12px;
  color: #666;
}

.device-meta {
  flex: 1;
  margin-bottom: 16px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.meta-row:last-child {
  margin-bottom: 0;
}

.meta-label {
  font-size: 12px;
  color: #999;
}

.meta-value {
  font-size: 12px;
  color: #666;
  font-weight: 500;
}

.device-actions {
  display: flex;
  gap: 8px;
  margin-top: auto;
}

.device-actions .el-button {
  flex: 1;
}

.device-dialog-content {
  text-align: center;
  padding: 20px 0;
}

.device-dialog-content .device-icon {
  margin: 0 auto 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.device-tip {
  margin-bottom: 20px;
  color: #666;
  font-size: 14px;
}

.dialog-footer {
  display: flex;
  justify-content: center;
  gap: 12px;
}

.dialog-footer .el-button {
  min-width: 80px;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
    gap: 15px;
  }
  
  .header-left {
    justify-content: flex-start;
  }
  
  .header-right {
    align-self: flex-end;
  }
  
  .devices-grid .el-col {
    margin-bottom: 16px;
  }
}
</style>