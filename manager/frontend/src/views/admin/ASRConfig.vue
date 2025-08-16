<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>ASR配置管理</h2>
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
      <el-table-column prop="is_default" label="默认配置" width="100">
        <template #default="scope">
          <el-tag :type="scope.row.is_default ? 'success' : 'info'">
            {{ scope.row.is_default ? '是' : '否' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
        <template #default="scope">
          {{ formatDate(scope.row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200">
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
      :title="editingConfig ? '编辑ASR配置' : '添加ASR配置'"
      width="600px"
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
            <el-option label="FunASR" value="funasr" />
          </el-select>
        </el-form-item>
        
        <el-form-item label="是否默认" prop="is_default">
          <el-switch v-model="form.is_default" />
        </el-form-item>
        
        <!-- FunASR配置字段 -->
        <div v-if="form.provider === 'funasr'">
          <el-form-item label="主机地址" prop="funasr.host">
            <el-input v-model="form.funasr.host" placeholder="请输入主机地址" />
          </el-form-item>
          
          <el-form-item label="端口" prop="funasr.port">
            <el-input-number v-model="form.funasr.port" :min="1" :max="65535" style="width: 100%" />
          </el-form-item>
          
          <el-form-item label="模式" prop="funasr.mode">
            <el-select v-model="form.funasr.mode" placeholder="请选择模式" style="width: 100%">
              <el-option label="2pass" value="2pass" />
              <el-option label="offline" value="offline" />
              <el-option label="online" value="online" />
            </el-select>
          </el-form-item>
          
          <el-form-item label="采样率" prop="funasr.sample_rate">
            <el-select v-model="form.funasr.sample_rate" placeholder="请选择采样率" style="width: 100%">
              <el-option label="8000" :value="8000" />
              <el-option label="16000" :value="16000" />
              <el-option label="44100" :value="44100" />
              <el-option label="48000" :value="48000" />
            </el-select>
          </el-form-item>
          
          <el-form-item label="块大小" prop="funasr.chunk_size">
            <el-input-number v-model="form.funasr.chunk_size" :min="1" style="width: 100%" />
          </el-form-item>
          
          <el-form-item label="块间隔" prop="funasr.chunk_interval">
            <el-input-number v-model="form.funasr.chunk_interval" :min="1" style="width: 100%" />
          </el-form-item>
          
          <el-form-item label="最大连接数" prop="funasr.max_connections">
            <el-input-number v-model="form.funasr.max_connections" :min="1" style="width: 100%" />
          </el-form-item>
          
          <el-form-item label="超时时间(秒)" prop="funasr.timeout">
            <el-input-number v-model="form.funasr.timeout" :min="1" style="width: 100%" />
          </el-form-item>
          
          <el-form-item label="自动结束" prop="funasr.auto_end">
            <el-switch v-model="form.funasr.auto_end" />
          </el-form-item>
        </div>
      </el-form>
      
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
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
  provider: '',
  is_default: false,
  funasr: {
    host: 'localhost',
    port: 10095,
    mode: '2pass',
    sample_rate: 16000,
    chunk_size: 60,
    chunk_interval: 10,
    max_connections: 100,
    timeout: 30,
    auto_end: true
  }
})

// 根据provider生成配置JSON
const generateConfig = () => {
  if (form.provider === 'funasr') {
    return JSON.stringify({ funasr: form.funasr })
  }
  return '{}'
}

const rules = {
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
  'funasr.host': [{ required: true, message: '请输入主机地址', trigger: 'blur' }],
  'funasr.port': [{ required: true, message: '请输入端口', trigger: 'blur' }],
  'funasr.mode': [{ required: true, message: '请选择模式', trigger: 'change' }],
  'funasr.sample_rate': [{ required: true, message: '请选择采样率', trigger: 'change' }],
  'funasr.chunk_size': [{ required: true, message: '请输入块大小', trigger: 'blur' }],
  'funasr.chunk_interval': [{ required: true, message: '请输入块间隔', trigger: 'blur' }],
  'funasr.max_connections': [{ required: true, message: '请输入最大连接数', trigger: 'blur' }],
  'funasr.timeout': [{ required: true, message: '请输入超时时间', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/asr-configs')
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
  
  // 解析配置JSON并填充到对应字段
  try {
    const configObj = JSON.parse(config.config || '{}')
    if (configObj.funasr) {
      form.funasr = { ...form.funasr, ...configObj.funasr }
    }
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
          provider: form.provider,
          is_default: form.is_default,
          config: generateConfig()
        }
        
        if (editingConfig.value) {
          await api.put(`/admin/asr-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('配置更新成功')
        } else {
          await api.post('/admin/asr-configs', configData)
          ElMessage.success('配置创建成功')
        }
        
        showDialog.value = false
        resetForm()
        loadConfigs()
      } catch (error) {
        ElMessage.error('保存失败: ' + (error.response?.data?.message || error.message))
      } finally {
        saving.value = false
      }
    }
  })
}

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('确定要删除这个配置吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.delete(`/admin/asr-configs/${id}`)
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
  form.provider = ''
  form.is_default = false
  form.funasr = {
    host: 'localhost',
    port: 10095,
    mode: '2pass',
    sample_rate: 16000,
    chunk_size: 60,
    chunk_interval: 10,
    max_connections: 100,
    timeout: 30,
    auto_end: true
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