<template>
  <div class="setup-container">
    <div class="setup-card">
      <div class="setup-header">
        <h1>系统初始化</h1>
        <p>欢迎使用小智管理系统，请完成初始设置</p>
      </div>

      <!-- 检查状态 -->
      <div v-if="!initialized" class="setup-status">
        <div class="loading-spinner" v-if="checking">
          <div class="spinner"></div>
          <p>正在检查系统状态...</p>
        </div>
        
        <div v-else-if="needsSetup" class="setup-form">
          <h2>创建管理员账户</h2>
          <p>请设置管理员账户信息，用于系统管理</p>
          
          <form @submit.prevent="initializeSystem">
            <div class="form-group">
              <label for="username">管理员用户名</label>
              <input
                id="username"
                v-model="form.admin_username"
                type="text"
                required
                minlength="3"
                maxlength="50"
                placeholder="请输入管理员用户名"
              />
            </div>
            
            <div class="form-group">
              <label for="email">管理员邮箱</label>
              <input
                id="email"
                v-model="form.admin_email"
                type="email"
                required
                placeholder="请输入管理员邮箱"
              />
            </div>
            
            <div class="form-group">
              <label for="password">管理员密码</label>
              <input
                id="password"
                v-model="form.admin_password"
                type="password"
                required
                minlength="6"
                maxlength="100"
                placeholder="请输入管理员密码（至少6位）"
              />
            </div>
            
            <div class="form-group">
              <label for="confirmPassword">确认密码</label>
              <input
                id="confirmPassword"
                v-model="confirmPassword"
                type="password"
                required
                placeholder="请再次输入密码"
              />
            </div>
            
            <div class="error-message" v-if="errorMessage">
              {{ errorMessage }}
            </div>
            
            <button type="submit" :disabled="initializing" class="setup-btn">
              <span v-if="initializing">正在初始化...</span>
              <span v-else>开始初始化</span>
            </button>
          </form>
        </div>
        
        <div v-else class="setup-complete">
          <div class="success-icon">✅</div>
          <h2>系统已初始化</h2>
          <p>系统已完成初始化，请使用管理员账户登录</p>
          <router-link to="/login" class="login-btn">前往登录</router-link>
        </div>
      </div>
      
      <!-- 初始化成功 -->
      <div v-else class="setup-success">
        <div class="success-icon">🎉</div>
        <h2>初始化成功！</h2>
        <p>系统已成功初始化，管理员账户已创建</p>
        <div class="admin-info">
          <p><strong>用户名：</strong>{{ adminInfo.username }}</p>
          <p><strong>邮箱：</strong>{{ adminInfo.email }}</p>
        </div>
        <router-link to="/login" class="login-btn">前往登录</router-link>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '@/utils/api'

export default {
  name: 'Setup',
  setup() {
    const router = useRouter()
    const checking = ref(true)
    const needsSetup = ref(false)
    const initialized = ref(false)
    const initializing = ref(false)
    const errorMessage = ref('')
    
    const form = ref({
      admin_username: '',
      admin_email: '',
      admin_password: ''
    })
    
    const confirmPassword = ref('')
    const adminInfo = ref({})

    const checkSetupStatus = async () => {
      try {
        checking.value = true
        const response = await api.get('/setup/status')
        
        if (response.data.needs_setup) {
          needsSetup.value = true
        } else {
          // 系统已初始化，跳转到登录页
          router.push('/login')
        }
      } catch (error) {
        console.error('检查系统状态失败:', error)
        errorMessage.value = '检查系统状态失败，请刷新页面重试'
      } finally {
        checking.value = false
      }
    }

    const initializeSystem = async () => {
      // 验证密码确认
      if (form.value.admin_password !== confirmPassword.value) {
        errorMessage.value = '两次输入的密码不一致'
        return
      }

      try {
        initializing.value = true
        errorMessage.value = ''
        
        const response = await api.post('/setup/initialize', form.value)
        
        adminInfo.value = response.data.admin
        initialized.value = true
      } catch (error) {
        console.error('系统初始化失败:', error)
        if (error.response?.data?.error) {
          errorMessage.value = error.response.data.error
        } else {
          errorMessage.value = '系统初始化失败，请重试'
        }
      } finally {
        initializing.value = false
      }
    }

    onMounted(() => {
      checkSetupStatus()
    })

    return {
      checking,
      needsSetup,
      initialized,
      initializing,
      errorMessage,
      form,
      confirmPassword,
      adminInfo,
      initializeSystem
    }
  }
}
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.setup-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
  padding: 40px;
  max-width: 500px;
  width: 100%;
}

.setup-header {
  text-align: center;
  margin-bottom: 30px;
}

.setup-header h1 {
  color: #333;
  margin-bottom: 10px;
  font-size: 28px;
}

.setup-header p {
  color: #666;
  font-size: 16px;
}

.loading-spinner {
  text-align: center;
  padding: 40px 0;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid #f3f3f3;
  border-top: 4px solid #667eea;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 20px;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.setup-form h2 {
  color: #333;
  margin-bottom: 10px;
  text-align: center;
}

.setup-form p {
  color: #666;
  text-align: center;
  margin-bottom: 30px;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  color: #333;
  font-weight: 500;
}

.form-group input {
  width: 100%;
  padding: 12px 16px;
  border: 2px solid #e1e5e9;
  border-radius: 8px;
  font-size: 16px;
  transition: border-color 0.3s;
  box-sizing: border-box;
}

.form-group input:focus {
  outline: none;
  border-color: #667eea;
}

.error-message {
  color: #e74c3c;
  background: #fdf2f2;
  border: 1px solid #fecaca;
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 20px;
  font-size: 14px;
}

.setup-btn {
  width: 100%;
  padding: 14px;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.3s;
}

.setup-btn:hover:not(:disabled) {
  background: #5a6fd8;
}

.setup-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.setup-complete,
.setup-success {
  text-align: center;
  padding: 40px 0;
}

.success-icon {
  font-size: 48px;
  margin-bottom: 20px;
}

.setup-complete h2,
.setup-success h2 {
  color: #333;
  margin-bottom: 10px;
}

.setup-complete p,
.setup-success p {
  color: #666;
  margin-bottom: 30px;
}

.admin-info {
  background: #f8f9fa;
  padding: 20px;
  border-radius: 8px;
  margin-bottom: 30px;
  text-align: left;
}

.admin-info p {
  margin: 8px 0;
  color: #333;
}

.login-btn {
  display: inline-block;
  padding: 12px 24px;
  background: #667eea;
  color: white;
  text-decoration: none;
  border-radius: 8px;
  font-weight: 500;
  transition: background-color 0.3s;
}

.login-btn:hover {
  background: #5a6fd8;
}
</style>
