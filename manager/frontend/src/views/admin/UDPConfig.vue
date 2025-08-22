<template>
  <div class="udp-config">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon">
            <Connection />
          </el-icon>
          <h1 class="page-title">UDP配置管理</h1>
        </div>
        <p class="page-description">配置UDP连接参数和网络设置</p>
      </div>
    </div>

    <!-- 表单容器 -->
    <div class="form-container">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        class="config-form"
        v-loading="loading"
      >
        <!-- 基础配置卡片 -->
        <el-card class="config-card basic-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon">
                <Setting />
              </el-icon>
              <span class="card-title">基础配置</span>
            </div>
          </template>
          
          <div class="form-grid basic-form-grid">
            <el-form-item label="是否启用" prop="enabled" class="form-item">
              <el-switch v-model="form.enabled" />
            </el-form-item>
            
            <el-form-item label="监听主机" prop="listen_host" class="form-item">
              <el-input v-model="form.listen_host" placeholder="请输入监听主机地址" />
            </el-form-item>
            
            <el-form-item label="监听端口" prop="listen_port" class="form-item">
              <el-input-number v-model="form.listen_port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
          </div>
        </el-card>

        <!-- 外部连接配置卡片 -->
        <el-card class="config-card external-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon external-icon">
                <Link />
              </el-icon>
              <span class="card-title">外部连接配置</span>
            </div>
          </template>
          
          <div class="form-grid">
            <el-form-item label="外部主机" prop="external_host" class="form-item">
              <el-input v-model="form.external_host" placeholder="请输入外部主机地址" />
            </el-form-item>
            
            <el-form-item label="外部端口" prop="external_port" class="form-item">
              <el-input-number v-model="form.external_port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
          </div>
        </el-card>

        <!-- 操作按钮区域 -->
        <div class="action-section">
          <el-button 
            type="primary" 
            @click="handleSave" 
            :loading="saving"
            class="save-button"
            size="large"
          >
            保存配置
          </el-button>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Setting, Link } from '@element-plus/icons-vue'
import api from '../../utils/api'

const loading = ref(false)
const saving = ref(false)
const configId = ref(null)
const formRef = ref(null)

const form = ref({
  name: 'UDP配置',
  is_default: true,
  enabled: true,
  external_host: '192.168.0.208',
  external_port: 8990,
  listen_host: '0.0.0.0',
  listen_port: 8990
})

const generateConfig = () => {
  return JSON.stringify({
    enabled: form.enabled,
    external_host: form.external_host,
    external_port: form.external_port,
    listen_host: form.listen_host,
    listen_port: form.listen_port
  })
}

const rules = {
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  external_host: [{ required: true, message: '请输入外部主机地址', trigger: 'blur' }],
  external_port: [
    { required: true, message: '请输入外部端口号', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: '端口号必须在1-65535之间', trigger: 'blur' }
  ],
  listen_host: [{ required: true, message: '请输入监听主机地址', trigger: 'blur' }],
  listen_port: [
    { required: true, message: '请输入监听端口号', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: '端口号必须在1-65535之间', trigger: 'blur' }
  ]
}

const loadConfig = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/udp-configs')
    const configs = response.data.data || []
    if (configs.length > 0) {
      const config = configs[0]
      configId.value = config.id
      
      // 解析JSON配置
      let configData = {}
      try {
        configData = JSON.parse(config.json_data || '{}')
      } catch (e) {
        console.warn('解析配置JSON失败:', e)
      }
      
      form.value = {
        name: config.name,
        is_default: config.is_default,
        enabled: configData.enabled !== undefined ? configData.enabled : true,
        external_host: configData.external_host || '192.168.0.208',
        external_port: configData.external_port || 8990,
        listen_host: configData.listen_host || '0.0.0.0',
        listen_port: configData.listen_port || 8990
      }
    }
  } catch (error) {
    console.error('加载UDP配置失败:', error)
    ElMessage.error('加载UDP配置失败')
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!formRef.value) return
  
  try {
    await formRef.value.validate()
  } catch (error) {
    return
  }
  
  saving.value = true
  
  try {
    const configData = {
      enabled: form.value.enabled,
      external_host: form.value.external_host,
      external_port: form.value.external_port,
      listen_host: form.value.listen_host,
      listen_port: form.value.listen_port
    }
    
    const payload = {
      name: form.value.name,
      config_id: `udp_${form.value.name.replace(/[^a-zA-Z0-9]/g, '_').toLowerCase()}`,
      is_default: form.value.is_default,
      json_data: JSON.stringify(configData)
    }
    
    if (configId.value) {
      await api.put(`/admin/udp-configs/${configId.value}`, payload)
      ElMessage.success('更新配置成功')
    } else {
      const response = await api.post('/admin/udp-configs', payload)
      configId.value = response.data.data.id
      ElMessage.success('创建配置成功')
    }
  } catch (error) {
    console.error('保存配置失败:', error)
    ElMessage.error('保存配置失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.udp-config {
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

/* 表单容器 */
.form-container {
  max-width: 1200px;
  margin: 0 auto;
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* 配置卡片 */
.config-card {
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
  overflow: hidden;
}

.config-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 25px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
}

.external-config {
  border-left: 4px solid #e6a23c;
}

.basic-config {
  border-left: 4px solid #409eff;
}

/* 卡片头部 */
.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0;
}

.card-icon {
  font-size: 20px;
  color: #409eff;
}

.external-icon {
  color: #e6a23c;
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: #1f2937;
}

/* 表单网格 */
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 24px;
  padding: 24px;
}

/* 基础配置表单网格 - 换行显示 */
.basic-form-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 24px;
  padding: 24px;
}

.form-item {
  margin-bottom: 0;
}

/* Element Plus 组件深度样式 */
:deep(.el-form-item__label) {
  font-weight: 500;
  color: #374151;
  font-size: 14px;
}

:deep(.el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06);
  transition: all 0.2s ease;
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 3px rgba(64, 158, 255, 0.1);
}

:deep(.el-select .el-input__wrapper) {
  border-radius: 8px;
}

:deep(.el-input-number .el-input__wrapper) {
  border-radius: 8px;
}

:deep(.el-switch) {
  --el-switch-on-color: #409eff;
}

:deep(.el-card__header) {
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-bottom: 1px solid #e2e8f0;
  padding: 20px 24px;
}

:deep(.el-card__body) {
  padding: 0;
}

/* 操作按钮区域 */
.action-section {
  display: flex;
  justify-content: center;
  padding: 32px 0;
}

.save-button {
  padding: 12px 32px;
  font-size: 16px;
  font-weight: 500;
  border-radius: 8px;
  background: linear-gradient(135deg, #409eff 0%, #67c23a 100%);
  border: none;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
}

.save-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 25px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
}

/* 响应式设计 */
@media (max-width: 768px) {
  .udp-config {
    padding: 16px;
  }
  
  .page-title {
    font-size: 24px;
  }
  
  .title-icon {
    font-size: 28px;
  }
  
  .form-grid {
    grid-template-columns: 1fr;
    gap: 16px;
    padding: 16px;
  }
  
  .page-description {
    margin-left: 44px;
  }
}

@media (max-width: 480px) {
  .title-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .page-title {
    font-size: 20px;
  }
  
  .page-description {
    margin-left: 0;
  }
}
</style>