<template>
  <div class="admin-devices">
    <div class="page-header">
      <h2>设备管理</h2>
      <p class="page-subtitle">管理系统中的所有设备</p>
    </div>

    <div class="toolbar">
      <el-button type="primary" @click="openAddDialog">
        <el-icon><Plus /></el-icon>
        添加设备
      </el-button>
      <el-button @click="loadDevices">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <el-table :data="devices" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="device_code" label="激活码" width="150" />
      <el-table-column prop="device_name" label="设备名称" width="150" />
      <el-table-column prop="user_id" label="用户ID" width="100" />
      <el-table-column label="关联智能体" width="150">
        <template #default="{ row }">
          <span v-if="row.agent_id > 0">
            智能体 {{ row.agent_id }}
          </span>
          <el-tag v-else type="info" size="small">未分配</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="激活状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.activated ? 'success' : 'warning'">
            {{ row.activated ? '已激活' : '未激活' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="在线状态" width="100">
        <template #default="{ row }">
          <el-tag :type="isDeviceOnline(row.last_active_at) ? 'success' : 'danger'">
            {{ isDeviceOnline(row.last_active_at) ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="last_active_at" label="最后活跃时间" width="180">
        <template #default="{ row }">
          {{ row.last_active_at ? new Date(row.last_active_at).toLocaleString() : '从未活跃' }}
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
        <template #default="{ row }">
          {{ new Date(row.created_at).toLocaleString() }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="editDevice(row)">
            编辑
          </el-button>
          <el-button size="small" type="danger" @click="deleteDevice(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑设备对话框 -->
    <el-dialog
      v-model="showAddDialog"
      :title="editingDevice ? '编辑设备' : '添加设备'"
      width="500px"
    >
      <el-form :model="deviceForm" :rules="deviceRules" ref="deviceFormRef" label-width="100px">
        <el-form-item label="用户ID" prop="user_id">
          <el-input-number v-model="deviceForm.user_id" :min="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="激活码" prop="device_code">
          <el-input 
            v-model="deviceForm.device_code" 
            :placeholder="editingDevice ? '请输入激活码' : '请输入激活码（与设备名称二选一）'" 
          />
        </el-form-item>
        <el-form-item label="设备名称" prop="device_name">
          <el-input 
            v-model="deviceForm.device_name" 
            :placeholder="editingDevice ? '请输入设备名称' : '请输入设备名称（与设备代码二选一）'" 
          />
        </el-form-item>
        <el-form-item label="激活状态" prop="activated">
          <el-switch v-model="deviceForm.activated" />
        </el-form-item>
        <el-form-item label="关联智能体" prop="agent_id">
          <el-select v-model="deviceForm.agent_id" placeholder="请选择智能体" style="width: 100%" clearable>
            <el-option label="不关联智能体" :value="0" />
            <el-option 
              v-for="agent in agents" 
              :key="agent.id" 
              :label="`${agent.name} (用户${agent.user_id})`" 
              :value="agent.id" 
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="saveDevice" :loading="saving">
          {{ editingDevice ? '更新' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import api from '../../utils/api'
import { useAuthStore } from '../../stores/auth'

const devices = ref([])
const agents = ref([])
const loading = ref(false)
const showAddDialog = ref(false)
const editingDevice = ref(null)
const saving = ref(false)
const deviceFormRef = ref()
const authStore = useAuthStore()

const deviceForm = ref({
  user_id: authStore.user?.id || null,
  device_code: '',
  device_name: '',
  activated: true,
  agent_id: 0
})

const deviceRules = {
  user_id: [{ required: true, message: '请输入用户ID', trigger: 'blur' }],
  device_code: [
    {
      validator: (rule, value, callback) => {
        // 如果是编辑模式，激活码必填
        if (editingDevice.value) {
          if (!value) {
            callback(new Error('请输入激活码'))
          } else {
            callback()
          }
          return
        }
        
        // 如果是新增模式，激活码和设备名称至少填一个
        if (!value && !deviceForm.value.device_name) {
          callback(new Error('激活码和设备名称至少填写一个'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  device_name: [
    {
      validator: (rule, value, callback) => {
        // 如果是编辑模式，设备名称必填
        if (editingDevice.value) {
          if (!value) {
            callback(new Error('请输入设备名称'))
          } else {
            callback()
          }
          return
        }
        
        // 如果是新增模式，激活码和设备名称至少填一个
        if (!value && !deviceForm.value.device_code) {
          callback(new Error('激活码和设备名称至少填写一个'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const loadDevices = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/devices')
    devices.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载设备列表失败')
    console.error('Error loading devices:', error)
  } finally {
    loading.value = false
  }
}

const loadAgents = async () => {
  try {
    const response = await api.get('/admin/agents')
    agents.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载智能体列表失败')
    console.error('Error loading agents:', error)
  }
}

const openAddDialog = () => {
  editingDevice.value = null
  deviceForm.value = {
    user_id: authStore.user?.id || null,
    device_code: '',
    device_name: '',
    activated: true,
    agent_id: 0
  }
  showAddDialog.value = true
}

// 验证激活码是否存在
const validateDeviceCode = async (deviceCode) => {
  if (!deviceCode) return null
  
  try {
    const response = await api.get(`/admin/devices/validate-code?code=${deviceCode}`)
    return response.data.exists
  } catch (error) {
    console.error('验证激活码失败:', error)
    return null
  }
}

const editDevice = (device) => {
  editingDevice.value = device
  deviceForm.value = {
    user_id: device.user_id,
    device_code: device.device_code,
    device_name: device.device_name,
    activated: device.activated,
    agent_id: device.agent_id || 0
  }
  showAddDialog.value = true
}

const saveDevice = async () => {
  if (!deviceFormRef.value) return
  
  const valid = await deviceFormRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    if (editingDevice.value) {
      await api.put(`/admin/devices/${editingDevice.value.id}`, deviceForm.value)
      ElMessage.success('设备更新成功')
    } else {
      const response = await api.post('/admin/devices', deviceForm.value)
      // 根据后端返回的消息显示不同的提示
      const message = response.data.message || '设备添加成功'
      ElMessage.success(message)
    }
    showAddDialog.value = false
    resetForm()
    loadDevices()
  } catch (error) {
    const errorMessage = error.response?.data?.error || (editingDevice.value ? '设备更新失败' : '设备添加失败')
    ElMessage.error(errorMessage)
    console.error('Error saving device:', error)
  } finally {
    saving.value = false
  }
}

const deleteDevice = async (device) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除设备 "${device.device_name}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    await api.delete(`/admin/devices/${device.id}`)
    ElMessage.success('设备删除成功')
    loadDevices()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('设备删除失败')
      console.error('Error deleting device:', error)
    }
  }
}

const resetForm = () => {
  editingDevice.value = null
  deviceForm.value = {
    user_id: authStore.user?.id || null,
    device_code: '',
    device_name: '',
    activated: true,
    agent_id: 0
  }
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
}

// 判断设备是否在线（基于最后活跃时间）
const isDeviceOnline = (lastActiveAt) => {
  if (!lastActiveAt) return false
  const now = new Date()
  const lastActive = new Date(lastActiveAt)
  // 5分钟内有活动认为在线
  return (now - lastActive) < 5 * 60 * 1000
}

onMounted(() => {
  loadDevices()
  loadAgents()
})
</script>

<style scoped>
.admin-devices {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0 0 8px 0;
  color: #303133;
  font-size: 24px;
  font-weight: 600;
}

.page-subtitle {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.toolbar {
  margin-bottom: 20px;
  display: flex;
  gap: 12px;
}
</style>