<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>VLLM配置管理</h2>
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
      <el-table-column prop="provider" label="提供商" />
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
      :title="editingConfig ? '编辑VLLM配置' : '添加VLLM配置'"
      width="700px"
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
        
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="form.provider" placeholder="请选择提供商" style="width: 100%">
            <el-option label="阿里云视觉" value="aliyun_vision" />
            <el-option label="豆包视觉" value="doubao_vision" />
          </el-select>
        </el-form-item>
        
        <!-- 移除是否默认开关，现在在列表页操作 -->
        
        <el-form-item label="类型" prop="type">
          <el-input v-model="form.type" placeholder="请输入类型" />
        </el-form-item>
        
        <el-form-item label="模型名称" prop="model_name">
          <el-input v-model="form.model_name" placeholder="请输入模型名称" />
        </el-form-item>
        
        <el-form-item label="API密钥" prop="api_key">
          <el-input v-model="form.api_key" type="password" placeholder="请输入API密钥" show-password />
        </el-form-item>
        
        <el-form-item label="基础URL" prop="base_url">
          <el-input v-model="form.base_url" placeholder="请输入基础URL" />
        </el-form-item>
        
        <el-form-item label="最大令牌数" prop="max_tokens">
          <el-input-number v-model="form.max_tokens" :min="1" :max="100000" placeholder="请输入最大令牌数" style="width: 100%" />
        </el-form-item>
        
        <el-form-item label="温度" prop="temperature">
          <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" placeholder="请输入温度" style="width: 100%" />
        </el-form-item>
        
        <el-form-item label="Top P" prop="top_p">
          <el-input-number v-model="form.top_p" :min="0" :max="1" :step="0.1" placeholder="请输入Top P" style="width: 100%" />
        </el-form-item>
        
        <el-form-item label="超时时间(秒)" prop="timeout">
          <el-input-number v-model="form.timeout" :min="1" :max="300" placeholder="请输入超时时间" style="width: 100%" />
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
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

const configs = ref([])
const loading = ref(false)
const saving = ref(false)
const showDialog = ref(false)
const editingConfig = ref(null)
const formRef = ref()

const form = reactive({
  name: '',
  provider: 'aliyun_vision',
  is_default: false,
  enabled: true,
  type: 'vllm',
  model_name: 'qwen-vl-max',
  api_key: '',
  base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  max_tokens: 1000,
  temperature: 0.1,
  top_p: 0.1,
  timeout: 30
})

const generateConfig = () => {
  return JSON.stringify({
    type: form.type,
    model_name: form.model_name,
    api_key: form.api_key,
    base_url: form.base_url,
    max_tokens: form.max_tokens,
    temperature: form.temperature,
    top_p: form.top_p,
    timeout: form.timeout
  })
}

const rules = {
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
  type: [{ required: true, message: '请输入类型', trigger: 'blur' }],
  model_name: [{ required: true, message: '请输入模型名称', trigger: 'blur' }],
  api_key: [{ required: true, message: '请输入API密钥', trigger: 'blur' }],
  base_url: [
    { required: true, message: '请输入基础URL', trigger: 'blur' },
    { type: 'url', message: '请输入有效的URL', trigger: 'blur' }
  ],
  max_tokens: [{ required: true, message: '请输入最大令牌数', trigger: 'blur' }],
  timeout: [{ required: true, message: '请输入超时时间', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/vllm-configs')
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
  form.provider = config.provider
  form.is_default = config.is_default
  form.enabled = config.enabled
  
  try {
    const configData = JSON.parse(config.json_data || '{}')
    form.type = configData.type || ''
    form.model_name = configData.model_name || ''
    form.api_key = configData.api_key || ''
    form.base_url = configData.base_url || ''
    form.max_tokens = configData.max_tokens || 4096
    form.temperature = configData.temperature !== undefined ? configData.temperature : 0.7
    form.top_p = configData.top_p !== undefined ? configData.top_p : 0.9
    form.timeout = configData.timeout || 30
  } catch (error) {
    console.error('解析配置失败:', error)
    ElMessage.warning('配置格式错误，已重置为默认值')
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
          provider: form.provider,
          is_default: false, // 新配置默认不是默认配置，可在列表页设置
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: generateConfig()
        }
        
        if (editingConfig.value) {
          await api.put(`/admin/vllm-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('更新成功')
        } else {
          await api.post('/admin/vllm-configs', configData)
          ElMessage.success('添加成功')
        }
        
        showDialog.value = false
        loadConfigs()
      } catch (error) {
        ElMessage.error('保存失败，请检查网络连接和输入内容')
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
      provider: config.provider,
      is_default: config.is_default,
      enabled: config.enabled,
      json_data: config.json_data
    }
    
    await api.put(`/admin/vllm-configs/${config.id}`, configData)
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
    
    await api.delete(`/admin/vllm-configs/${id}`)
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
  Object.assign(form, {
    name: '',
    provider: 'aliyun_vision',
    is_default: false,
    enabled: true,
    type: 'vllm',
    model_name: 'qwen-vl-max',
    api_key: '',
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    max_tokens: 1000,
    temperature: 0.1,
    top_p: 0.1,
    timeout: 30
  })
  formRef.value?.clearValidate()
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

.config-example {
  margin-top: 8px;
}
</style>