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
      <el-table-column prop="name" label="名称" width="150" />
      <el-table-column prop="user_id" label="用户ID" width="100" />
      <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">
            {{ row.status === 'active' ? '活跃' : '非活跃' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
        <template #default="{ row }">
          {{ new Date(row.created_at).toLocaleString() }}
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" label="更新时间" width="180">
        <template #default="{ row }">
          {{ new Date(row.updated_at).toLocaleString() }}
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
      <el-form :model="agentForm" :rules="agentRules" ref="agentFormRef" label-width="100px">
        <el-form-item label="用户ID" prop="user_id">
          <el-input-number v-model="agentForm.user_id" :min="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="agentForm.name" placeholder="请输入智能体名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="agentForm.description"
            type="textarea"
            :rows="3"
            placeholder="请输入智能体描述"
          />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="agentForm.status" style="width: 100%">
            <el-option label="活跃" value="active" />
            <el-option label="非活跃" value="inactive" />
          </el-select>
        </el-form-item>
        <el-form-item label="系统提示" prop="system_prompt">
          <el-input
            v-model="agentForm.system_prompt"
            type="textarea"
            :rows="4"
            placeholder="请输入系统提示词"
          />
        </el-form-item>
        <el-form-item label="配置" prop="config">
          <el-input
            v-model="agentForm.config"
            type="textarea"
            :rows="3"
            placeholder="请输入JSON格式的配置"
          />
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
const loading = ref(false)
const showAddDialog = ref(false)
const editingAgent = ref(null)
const saving = ref(false)
const agentFormRef = ref()

const agentForm = ref({
  user_id: null,
  name: '',
  description: '',
  status: 'active',
  system_prompt: '',
  config: '{}'
})

const agentRules = {
  user_id: [{ required: true, message: '请输入用户ID', trigger: 'blur' }],
  name: [{ required: true, message: '请输入智能体名称', trigger: 'blur' }],
  description: [{ required: true, message: '请输入描述', trigger: 'blur' }],
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

const editAgent = (agent) => {
  editingAgent.value = agent
  agentForm.value = {
    user_id: agent.user_id,
    name: agent.name,
    description: agent.description,
    status: agent.status,
    system_prompt: agent.system_prompt || '',
    config: agent.config || '{}'
  }
  showAddDialog.value = true
}

const saveAgent = async () => {
  if (!agentFormRef.value) return
  
  const valid = await agentFormRef.value.validate().catch(() => false)
  if (!valid) return

  // 验证JSON格式
  try {
    JSON.parse(agentForm.value.config)
  } catch (error) {
    ElMessage.error('配置必须是有效的JSON格式')
    return
  }

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
    description: '',
    status: 'active',
    system_prompt: '',
    config: '{}'
  }
  if (agentFormRef.value) {
    agentFormRef.value.resetFields()
  }
}

onMounted(() => {
  loadAgents()
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