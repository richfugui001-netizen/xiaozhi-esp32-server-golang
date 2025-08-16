<template>
  <div class="mqtt-server-config">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon">
            <Monitor />
          </el-icon>
          <h1 class="page-title">MQTT Server配置管理</h1>
        </div>
        <p class="page-description">配置MQTT服务器参数和安全设置</p>
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
            <el-form-item label="启用状态" prop="enable" class="form-item">
              <el-switch v-model="form.enable" />
            </el-form-item>
            
            <el-form-item label="监听主机" prop="listen_host" class="form-item">
              <el-input v-model="form.listen_host" placeholder="请输入监听主机地址" style="max-width: 300px" />
            </el-form-item>
            
            <el-form-item label="监听端口" prop="listen_port" class="form-item">
              <el-input-number v-model="form.listen_port" :min="1" :max="65535" placeholder="请输入监听端口号" style="max-width: 200px" />
            </el-form-item>
          </div>
        </el-card>

        <!-- 认证配置卡片 -->
        <el-card class="config-card auth-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon auth-icon">
                <User />
              </el-icon>
              <span class="card-title">认证配置</span>
            </div>
          </template>
          
          <!-- 提示信息 -->
          <div class="config-tip">
            <el-icon class="tip-icon">
              <InfoFilled />
            </el-icon>
            <span class="tip-text">主程序连接mqtt server所使用的用户名密码</span>
          </div>
          
          <div class="form-grid auth-form-grid">
            <el-form-item label="启用认证" prop="enable_auth" class="form-item">
              <el-switch v-model="form.enable_auth" />
            </el-form-item>
            
            <div class="form-row">
              <el-form-item label="管理员用户" prop="username" class="form-item">
                <el-input v-model="form.username" placeholder="请输入管理员用户名" style="max-width: 250px" />
              </el-form-item>
              
              <el-form-item label="管理员密码" prop="password" class="form-item">
                <el-input v-model="form.password" type="password" placeholder="请输入管理员密码" show-password style="max-width: 250px" />
              </el-form-item>
            </div>
            
            <el-form-item label="签名密钥" prop="signature_key" class="form-item">
              <el-input v-model="form.signature_key" placeholder="请输入签名密钥" style="max-width: 400px" />
            </el-form-item>
          </div>
        </el-card>

        <!-- TLS配置卡片 -->
        <el-card class="config-card tls-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon tls-icon">
                <Lock />
              </el-icon>
              <span class="card-title">TLS配置</span>
            </div>
          </template>
          
          <div class="form-grid tls-form-grid">
            <div class="form-row">
              <el-form-item label="启用TLS" prop="tls.enable" class="form-item">
                <el-switch v-model="form.tls.enable" />
              </el-form-item>
              
              <el-form-item label="TLS端口" prop="tls.port" v-if="form.tls.enable" class="form-item">
                <el-input-number v-model="form.tls.port" :min="1" :max="65535" placeholder="请输入TLS端口号" style="max-width: 200px" />
              </el-form-item>
            </div>
            
            <el-form-item label="证书文件" prop="tls.pem" v-if="form.tls.enable" class="form-item">
              <el-input v-model="form.tls.pem" placeholder="请输入证书文件路径" style="max-width: 400px" />
            </el-form-item>
            
            <el-form-item label="密钥文件" prop="tls.key" v-if="form.tls.enable" class="form-item">
              <el-input v-model="form.tls.key" placeholder="请输入密钥文件路径" style="max-width: 400px" />
            </el-form-item>
          </div>
        </el-card>

        <!-- 操作按钮 -->
        <div class="action-section">
          <el-button type="primary" @click="handleSave" :loading="saving" class="save-button">
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
import { Monitor, Setting, Platform, User, Lock, InfoFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'

const loading = ref(false)
const saving = ref(false)
const configId = ref(null)
const formRef = ref(null)

const form = reactive({
  enable: true,
  listen_host: '0.0.0.0',
  listen_port: 1883,
  username: '',
  password: '',
  signature_key: '',
  enable_auth: false,
  tls: {
    enable: false,
    port: 8883,
    pem: '',
    key: ''
  }
})



const rules = {
  listen_host: [{ required: true, message: '请输入监听主机地址', trigger: 'blur' }],
  listen_port: [
    { required: true, message: '请输入监听端口号', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: '端口号必须在1-65535之间', trigger: 'blur' }
  ],
  'tls.port': [
    {
      validator: (rule, value, callback) => {
        if (form.tls.enable && (!value || value < 1 || value > 65535)) {
          callback(new Error('启用TLS时端口号必须在1-65535之间'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  'tls.pem': [
    {
      validator: (rule, value, callback) => {
        if (form.tls.enable && !value) {
          callback(new Error('启用TLS时证书文件路径不能为空'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  'tls.key': [
    {
      validator: (rule, value, callback) => {
        if (form.tls.enable && !value) {
          callback(new Error('启用TLS时密钥文件路径不能为空'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const loadConfig = async () => {
  try {
    loading.value = true
    const response = await api.get('/admin/mqtt-server-configs')
    if (response.data && response.data.length > 0) {
      const config = response.data[0]
      configId.value = config.id
      Object.assign(form, config)
    }
  } catch (error) {
    ElMessage.error('加载配置失败：' + error.message)
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!formRef.value) return
  
  try {
    await formRef.value.validate()
    saving.value = true
    
    const configData = {
      enable: form.enable,
      listen_host: form.listen_host,
      listen_port: form.listen_port,
      client_id: form.client_id,
      username: form.username,
      password: form.password,
      signature_key: form.signature_key,
      enable_auth: form.enable_auth,
      tls: {
        enable: form.tls.enable,
        port: form.tls.port,
        pem: form.tls.pem,
        key: form.tls.key
      }
    }
    
    const payload = {
      name: 'MQTT Server配置',
      is_default: true,
      config: JSON.stringify(configData)
    }
    
    if (configId.value) {
      await api.put(`/admin/mqtt-server-configs/${configId.value}`, payload)
      ElMessage.success('更新配置成功')
    } else {
      const response = await api.post('/admin/mqtt-server-configs', payload)
      configId.value = response.data.id
      ElMessage.success('创建配置成功')
    }
  } catch (error) {
    if (error.message) {
      ElMessage.error('保存失败：' + error.message)
    }
  } finally {
    saving.value = false
  }
}



onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.mqtt-server-config {
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

.basic-config {
  border-left: 4px solid #409eff;
}

.server-config {
  border-left: 4px solid #67c23a;
}

.auth-config {
  border-left: 4px solid #e6a23c;
}

.tls-config {
  border-left: 4px solid #f56c6c;
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

.server-icon {
  color: #67c23a;
}

.auth-icon {
  color: #e6a23c;
}

.tls-icon {
  color: #f56c6c;
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: #1f2937;
}

/* 配置提示 */
.config-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 24px;
  background: #f0f9ff;
  border-left: 4px solid #0ea5e9;
  margin-bottom: 16px;
}

.tip-icon {
  font-size: 16px;
  color: #0ea5e9;
  flex-shrink: 0;
}

.tip-text {
  font-size: 14px;
  color: #0369a1;
  line-height: 1.5;
}

/* 表单网格 */
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 24px;
  padding: 24px;
}

/* 基础配置表单网格 - 垂直布局 */
.basic-form-grid {
  grid-template-columns: 1fr;
  gap: 20px;
}

/* 认证配置表单网格 */
.auth-form-grid {
  grid-template-columns: 1fr;
  gap: 20px;
}

/* TLS配置表单网格 */
.tls-form-grid {
  grid-template-columns: 1fr;
  gap: 20px;
}

/* 表单行 - 水平布局 */
.form-row {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  flex-wrap: wrap;
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
  .mqtt-server-config {
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
  
  .form-row {
    flex-direction: column;
    gap: 16px;
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