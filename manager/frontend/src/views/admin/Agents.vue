<template>
  <div class="admin-agents">
    <div class="page-header">
      <h2>智能体管理</h2>
      <p class="page-subtitle">管理系统中的所有智能体</p>
    </div>

    <div class="toolbar">
      <el-button type="primary" @click="showAddDialog = true">
        <el-icon><Plus /></el-icon>
        添加智能体
      </el-button>
      <el-button @click="loadAgents">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <el-table :data="agents" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="昵称" width="150" />
      <el-table-column prop="user_id" label="用户ID" width="100" />
      <el-table-column label="角色介绍" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.custom_prompt || '未设置' }}
        </template>
      </el-table-column>
      <el-table-column label="语言模型" width="150">
        <template #default="{ row }">
          {{ row.llm_config?.name || '未设置' }}
        </template>
      </el-table-column>
      <el-table-column label="音色" width="150">
        <template #default="{ row }">
          {{ row.tts_config?.name || '未设置' }}
        </template>
      </el-table-column>
      <el-table-column label="语音识别速度" width="120">
        <template #default="{ row }">
          <el-tag :type="getASRSpeedType(row.asr_speed)">
            {{ getASRSpeedText(row.asr_speed) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">
            {{ row.status === 'active' ? '活跃' : '非活跃' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="editAgent(row)">
            编辑
          </el-button>
          <el-button size="small" type="danger" @click="deleteAgent(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑智能体对话框 -->
    <el-dialog
      v-model="showAddDialog"
      :title="editingAgent ? '编辑智能体' : '添加智能体'"
      width="600px"
    >
      <el-form :model="agentForm" :rules="agentRules" ref="agentFormRef" label-width="120px">
        <el-form-item label="用户ID" prop="user_id">
          <el-input-number v-model="agentForm.user_id" :min="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="昵称" prop="name">
          <el-input v-model="agentForm.name" placeholder="请输入智能体昵称" />
        </el-form-item>
        <el-form-item label="角色介绍" prop="custom_prompt">
          <el-input
            v-model="agentForm.custom_prompt"
            type="textarea"
            :rows="4"
            placeholder="请输入角色介绍/系统提示词"
          />
        </el-form-item>
        <el-form-item label="语言模型" prop="llm_config_id">
          <el-select v-model="agentForm.llm_config_id" placeholder="请选择语言模型" style="width: 100%">
            <el-option 
              v-for="config in llmConfigs" 
              :key="config.config_id" 
              :label="config.name" 
              :value="config.config_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="音色" prop="tts_config_id">
          <el-select v-model="agentForm.tts_config_id" placeholder="请选择音色" style="width: 100%">
            <el-option 
              v-for="config in ttsConfigs" 
              :key="config.config_id" 
              :label="config.name" 
              :value="config.config_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="语音识别速度" prop="asr_speed">
          <el-select v-model="agentForm.asr_speed" style="width: 100%">
            <el-option label="正常" value="normal" />
            <el-option label="耐心" value="patient" />
            <el-option label="快速" value="fast" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="agentForm.status" style="width: 100%">
            <el-option label="活跃" value="active" />
            <el-option label="非活跃" value="inactive" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="saveAgent" :loading="saving">
          {{ editingAgent ? '更新' : '添加' }}
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

const agents = ref([])
const llmConfigs = ref([])
const ttsConfigs = ref([])
const loading = ref(false)
const showAddDialog = ref(false)
const editingAgent = ref(null)
const saving = ref(false)
const agentFormRef = ref()

const agentForm = ref({
  user_id: null,
  name: '',
  custom_prompt: '',
  llm_config_id: null,
  tts_config_id: null,
  asr_speed: 'normal',
  status: 'active'
})

const agentRules = {
  user_id: [{ required: true, message: '请输入用户ID', trigger: 'blur' }],
  name: [{ required: true, message: '请输入智能体昵称', trigger: 'blur' }],
  asr_speed: [{ required: true, message: '请选择语音识别速度', trigger: 'change' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }]
}

const loadAgents = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/agents')
    agents.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载智能体列表失败')
    console.error('Error loading agents:', error)
  } finally {
    loading.value = false
  }
}

const loadConfigs = async () => {
  try {
    const [llmResponse, ttsResponse] = await Promise.all([
      api.get('/admin/llm-configs'),
      api.get('/admin/tts-configs')
    ])
    llmConfigs.value = llmResponse.data.data || []
    ttsConfigs.value = ttsResponse.data.data || []
    
    // 对配置进行排序，默认配置排在前面
    llmConfigs.value.sort((a, b) => {
      if (a.is_default && !b.is_default) return -1
      if (!a.is_default && b.is_default) return 1
      return a.name.localeCompare(b.name)
    })
    
    ttsConfigs.value.sort((a, b) => {
      if (a.is_default && !b.is_default) return -1
      if (!a.is_default && b.is_default) return 1
      return a.name.localeCompare(b.name)
    })
  } catch (error) {
    console.error('Error loading configs:', error)
  }
}

const editAgent = (agent) => {
  editingAgent.value = agent
  agentForm.value = {
    user_id: agent.user_id,
    name: agent.name,
    custom_prompt: agent.custom_prompt || '',
    llm_config_id: agent.llm_config_id,
    tts_config_id: agent.tts_config_id,
    asr_speed: agent.asr_speed || 'normal',
    status: agent.status
  }
  showAddDialog.value = true
}

const saveAgent = async () => {
  if (!agentFormRef.value) return
  
  const valid = await agentFormRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    if (editingAgent.value) {
      await api.put(`/admin/agents/${editingAgent.value.id}`, agentForm.value)
      ElMessage.success('智能体更新成功')
    } else {
      await api.post('/admin/agents', agentForm.value)
      ElMessage.success('智能体添加成功')
    }
    showAddDialog.value = false
    resetForm()
    loadAgents()
  } catch (error) {
    ElMessage.error(editingAgent.value ? '智能体更新失败' : '智能体添加失败')
    console.error('Error saving agent:', error)
  } finally {
    saving.value = false
  }
}

const deleteAgent = async (agent) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除智能体 "${agent.name}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    await api.delete(`/admin/agents/${agent.id}`)
    ElMessage.success('智能体删除成功')
    loadAgents()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('智能体删除失败')
      console.error('Error deleting agent:', error)
    }
  }
}

const resetForm = () => {
  editingAgent.value = null
  agentForm.value = {
    user_id: null,
    name: '',
    custom_prompt: '',
    llm_config_id: null,
    tts_config_id: null,
    asr_speed: 'normal',
    status: 'active'
  }
  
  // 为新建智能体自动选择默认配置
  if (!editingAgent.value) {
    const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
    const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
    
    if (defaultLlmConfig) {
      agentForm.value.llm_config_id = defaultLlmConfig.config_id
    }
    if (defaultTtsConfig) {
      agentForm.value.tts_config_id = defaultTtsConfig.config_id
    }
  }
  
  if (agentFormRef.value) {
    agentFormRef.value.resetFields()
  }
}

const getASRSpeedText = (speed) => {
  const speedMap = {
    'normal': '正常',
    'patient': '耐心',
    'fast': '快速'
  }
  return speedMap[speed] || '正常'
}

const getASRSpeedType = (speed) => {
  const typeMap = {
    'normal': '',
    'patient': 'warning',
    'fast': 'success'
  }
  return typeMap[speed] || ''
}

onMounted(() => {
  loadAgents()
  loadConfigs()
})
</script>

<style scoped>
.admin-agents {
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