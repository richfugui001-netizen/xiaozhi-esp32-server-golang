<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>LLM配置管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showDialog = true">
          <el-icon><Plus /></el-icon>
          添加配置
        </el-button>
      </div>
    </div>

    <el-table :data="configs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="配置名称" />
      <el-table-column prop="config_id" label="配置ID" width="150" />
      <el-table-column prop="provider" label="类型" />
      <el-table-column prop="enabled" label="启用状态" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.enabled" 
            @change="toggleEnable(scope.row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="默认配置" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.is_default" 
            @change="toggleDefault(scope.row)"
            :disabled="scope.row.is_default && getEnabledConfigs().length === 1"
          />
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
        <template #default="scope">
          {{ formatDate(scope.row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="scope">
          <el-button size="small" @click="editConfig(scope.row)">编辑</el-button>
          <el-button
            size="small"
            type="danger"
            @click="deleteConfig(scope.row.id)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑配置弹窗 -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? '编辑LLM配置' : '添加LLM配置'"
      width="600px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="配置名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入配置名称" />
        </el-form-item>
        
        <el-form-item label="配置ID" prop="config_id">
          <el-input v-model="form.config_id" placeholder="请输入唯一的配置ID" />
        </el-form-item>
        
        <el-form-item label="类型" prop="provider">
          <el-select v-model="form.provider" placeholder="请选择类型" style="width: 100%">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Ollama" value="ollama" />
            <el-option label="Eino LLM" value="eino_llm" />
            <el-option label="Eino" value="eino" />
          </el-select>
        </el-form-item>
        
        <!-- 移除是否默认开关，现在在列表页操作 -->
        
        <!-- 通用配置字段 -->
        <el-form-item label="模型类型" prop="type">
          <el-select v-model="form.type" placeholder="请选择模型类型" style="width: 100%">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Ollama" value="ollama" />
          </el-select>
        </el-form-item>
        
        <el-form-item label="模型名称" prop="model_name">
          <el-input v-model="form.model_name" placeholder="请输入模型名称" />
        </el-form-item>
        
        <el-form-item label="API密钥" prop="api_key">
          <el-input v-model="form.api_key" type="password" placeholder="请输入API密钥" show-password />
        </el-form-item>
        
        <el-form-item label="基础URL" prop="base_url">
          <el-input v-model="form.base_url" placeholder="请输入基础URL" style="width: 100%;" />
        </el-form-item>
        
        <el-form-item label="">
          <el-dropdown @command="fillQuickUrl" trigger="click" style="width: 100%;">
            <el-button type="primary" plain style="width: 100%;">
              快捷URL填写 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="openai">OpenAI</el-dropdown-item>
                <el-dropdown-item command="azure">Azure OpenAI</el-dropdown-item>
                <el-dropdown-item command="anthropic">Anthropic</el-dropdown-item>
                <el-dropdown-item command="zhipu">智谱AI</el-dropdown-item>
                <el-dropdown-item command="aliyun">阿里云</el-dropdown-item>
                <el-dropdown-item command="doubao">豆包</el-dropdown-item>
                <el-dropdown-item command="siliconflow">SiliconFlow</el-dropdown-item>
                <el-dropdown-item command="deepseek">DeepSeek</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </el-form-item>
        
        <el-form-item label="max_tokens" prop="max_tokens">
          <el-input-number v-model="form.max_tokens" :min="1" :max="100000" placeholder="max_tokens" style="width: 100%" />
        </el-form-item>
        
        <!-- 可选的高级配置 -->
        <el-form-item label="温度" prop="temperature">
          <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" placeholder="温度" style="width: 100%" />
        </el-form-item>
        
        <el-form-item label="Top P" prop="top_p">
          <el-input-number v-model="form.top_p" :min="0" :max="1" :step="0.1" placeholder="Top P" style="width: 100%" />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="handleDialogClose">取消</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, ArrowDown } from '@element-plus/icons-vue'
import api from '../../utils/api'

const configs = ref([])
const loading = ref(false)
const saving = ref(false)
const showDialog = ref(false)
const editingConfig = ref(null)
const formRef = ref()

const form = reactive({
  name: '',
  config_id: '',
  provider: '',
  is_default: false,
  enabled: true,
  type: 'openai',
  model_name: 'gpt-3.5-turbo',
  api_key: '',
  base_url: 'https://api.openai.com/v1',
  max_tokens: 4000,
  temperature: 0.7,
  top_p: 0.9
})

// 快捷URL填写功能
const quickUrls = {
  openai: 'https://api.openai.com/v1',
  azure: 'https://your-resource-name.openai.azure.com',
  anthropic: 'https://api.anthropic.com',
  zhipu: 'https://open.bigmodel.cn/api/paas/v4',
  aliyun: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  doubao: 'https://ark.cn-beijing.volces.com/api/v3',
  siliconflow: 'https://api.siliconflow.cn/v1',
  deepseek: 'https://api.deepseek.com/v1'
}

const fillQuickUrl = (command) => {
  if (quickUrls[command]) {
    form.base_url = quickUrls[command]
  }
}

// 生成配置JSON字符串
const generateConfig = () => {
  const config = {
    type: form.type,
    model_name: form.model_name,
    api_key: form.api_key,
    base_url: form.base_url,
    max_tokens: form.max_tokens
  }
  
  // 添加可选的高级配置
  if (form.temperature !== undefined && form.temperature !== null) {
    config.temperature = form.temperature
  }
  if (form.top_p !== undefined && form.top_p !== null) {
    config.top_p = form.top_p
  }
  
  return JSON.stringify(config, null, 2)
}

const rules = {
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  config_id: [{ required: true, message: '请输入配置ID', trigger: 'blur' }],
  provider: [{ required: false, message: '请选择类型', trigger: 'change' }],
  type: [{ required: true, message: '请选择模型类型', trigger: 'change' }],
  model_name: [{ required: true, message: '请输入模型名称', trigger: 'blur' }],
  api_key: [{ required: true, message: '请输入API密钥', trigger: 'blur' }],
  base_url: [{ required: true, message: '请输入基础URL', trigger: 'blur' }],
  max_tokens: [{ required: true, message: '请输入max_tokens', trigger: 'blur' }, { type: 'number', min: 1, max: 100000, message: 'max_tokens必须在1-100000之间', trigger: 'blur' }],
  temperature: [{ type: 'number', min: 0, max: 2, message: '温度必须在0-2之间', trigger: 'blur' }],
  top_p: [{ type: 'number', min: 0, max: 1, message: 'Top P必须在0-1之间', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/llm-configs')
    configs.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

const editConfig = (config) => {
  editingConfig.value = config
  form.name = config.name
  form.config_id = config.config_id
  form.provider = config.provider
  form.is_default = config.is_default
  form.enabled = config.enabled
  
  // 解析配置JSON并填充到对应字段
  try {
    const configObj = JSON.parse(config.json_data || '{}')
    form.type = configObj.type || 'openai'
    form.model_name = configObj.model_name || ''
    form.api_key = configObj.api_key || ''
    form.base_url = configObj.base_url || ''
    form.max_tokens = configObj.max_tokens || 4000
    form.temperature = configObj.temperature || 0.7
    form.top_p = configObj.top_p || 0.9
  } catch (error) {
    console.error('解析配置JSON失败:', error)
  }
  
  showDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: false, // 新配置默认不是默认配置，可在列表页设置
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: generateConfig()
        }
        
        if (editingConfig.value) {
          await api.put(`/admin/llm-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('配置更新成功')
        } else {
          await api.post('/admin/llm-configs', configData)
          ElMessage.success('配置创建成功')
        }
        
        showDialog.value = false
        loadConfigs()
      } catch (error) {
        ElMessage.error('保存失败: ' + (error.response?.data?.message || error.message))
      } finally {
        saving.value = false
      }
    }
  })
}

const toggleEnable = async (config) => {
  try {
    await api.post(`/admin/configs/${config.id}/toggle`)
    ElMessage.success(`${config.enabled ? '启用' : '禁用'}成功`)
  } catch (error) {
    // 恢复开关状态
    config.enabled = !config.enabled
    ElMessage.error('操作失败')
  }
}

const toggleDefault = async (config) => {
  try {
    if (!config.enabled) {
      ElMessage.warning('请先启用该配置才能设为默认')
      config.is_default = false
      return
    }
    
    const configData = {
      name: config.name,
      config_id: config.config_id,
      provider: config.provider,
      is_default: config.is_default,
      enabled: config.enabled,
      json_data: config.json_data
    }
    
    await api.put(`/admin/llm-configs/${config.id}`, configData)
    ElMessage.success(config.is_default ? '设为默认成功' : '取消默认成功')
    
    // 刷新列表以更新其他配置的默认状态
    loadConfigs()
  } catch (error) {
    // 恢复开关状态
    config.is_default = !config.is_default
    ElMessage.error('操作失败')
  }
}

const getEnabledConfigs = () => {
  return configs.value.filter(config => config.enabled)
}

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('确定要删除这个配置吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.delete(`/admin/llm-configs/${id}`)
    ElMessage.success('删除成功')
    loadConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const resetForm = () => {
  editingConfig.value = null
  form.name = ''
  form.config_id = ''
  form.provider = ''
  form.is_default = false
  form.enabled = true
  form.type = 'openai'
  form.model_name = 'gpt-3.5-turbo'
  form.api_key = ''
  form.base_url = 'https://api.openai.com/v1'
  form.max_tokens = 4000
  form.temperature = 0.7
  form.top_p = 0.9
}

const handleDialogClose = () => {
  showDialog.value = false
  resetForm()
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadConfigs()
})
</script>

<style scoped>
.config-page {
  padding: 20px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-left h2 {
  margin: 0;
  color: #333;
}
</style>