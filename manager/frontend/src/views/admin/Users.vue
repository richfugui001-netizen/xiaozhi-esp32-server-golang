<template>
  <div class="config-page">
    <!-- 页面标题和操作按钮 -->
    <div class="page-header">
      <div class="header-left">
        <h2>用户管理</h2>
      </div>
      <div class="header-right">
        <el-input
          v-model="searchKeyword"
          placeholder="搜索用户..."
          style="width: 200px; margin-right: 10px"
          prefix-icon="Search"
          clearable
        />
        <el-button type="primary" @click="openAddDialog">
          <el-icon><Plus /></el-icon>
          添加用户
        </el-button>
      </div>
    </div>

    <!-- 用户列表表格 -->
    <el-table :data="filteredUserList" v-loading="tableLoading" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="用户名" width="150" />
      <el-table-column prop="email" label="邮箱" width="200" />
      <el-table-column prop="role" label="角色" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'">
            {{ row.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
        <template #default="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="280">
        <template #default="{ row }">
          <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
          <el-button size="small" type="warning" @click="openResetPasswordDialog(row)">
            重置密码
          </el-button>
          <el-button 
            size="small" 
            type="danger" 
            @click="handleDeleteUser(row)"
            :disabled="row.role === 'admin'"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑用户对话框 -->
    <el-dialog 
      v-model="userDialogVisible" 
      :title="isEditMode ? '编辑用户' : '添加用户'"
      width="500px"
      @close="resetUserForm"
    >
      <el-form 
        ref="userFormRef" 
        :model="userForm" 
        :rules="userFormRules" 
        label-width="80px"
      >
        <el-form-item label="用户名" prop="username">
          <el-input 
            v-model="userForm.username" 
            :disabled="isEditMode"
            placeholder="请输入用户名"
          />
        </el-form-item>
        
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="userForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        
        <el-form-item v-if="!isEditMode" label="密码" prop="password">
          <el-input 
            v-model="userForm.password" 
            type="password" 
            placeholder="请输入密码（至少6位）"
            show-password
          />
        </el-form-item>
        
        <el-form-item label="角色" prop="role">
          <el-select v-model="userForm.role" placeholder="请选择角色" style="width: 100%">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="userDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleUserSubmit" :loading="userSubmitLoading">
          {{ isEditMode ? '保存' : '添加' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 重置密码对话框 -->
    <el-dialog 
      v-model="resetPasswordDialogVisible" 
      title="重置密码" 
      width="400px"
      @close="resetPasswordForm"
    >
      <el-form 
        ref="passwordFormRef" 
        :model="passwordForm" 
        :rules="passwordFormRules" 
        label-width="80px"
      >
        <el-form-item label="用户">
          <el-input v-model="currentUser.username" disabled />
        </el-form-item>
        
        <el-form-item label="新密码" prop="newPassword">
          <el-input 
            v-model="passwordForm.newPassword" 
            type="password" 
            placeholder="请输入新密码（至少6位）"
            show-password
          />
        </el-form-item>
        
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input 
            v-model="passwordForm.confirmPassword" 
            type="password" 
            placeholder="请再次输入新密码"
            show-password
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="resetPasswordDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleResetPassword" :loading="resetPasswordLoading">
          确认重置
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

// 数据状态
const userList = ref([])
const tableLoading = ref(false)
const userDialogVisible = ref(false)
const resetPasswordDialogVisible = ref(false)
const userSubmitLoading = ref(false)
const resetPasswordLoading = ref(false)
const isEditMode = ref(false)
const currentUser = ref({})
const searchKeyword = ref('')

// 计算属性
const filteredUserList = computed(() => {
  if (!searchKeyword.value) {
    return userList.value
  }
  return userList.value.filter(user => 
    user.username.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    user.email.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

// 表单引用
const userFormRef = ref()
const passwordFormRef = ref()

// 用户表单数据
const userForm = reactive({
  username: '',
  email: '',
  password: '',
  role: ''
})

// 密码表单数据
const passwordForm = reactive({
  newPassword: '',
  confirmPassword: ''
})

// 用户表单验证规则
const userFormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6位', trigger: 'blur' }
  ],
  role: [
    { required: true, message: '请选择角色', trigger: 'change' }
  ]
}

// 密码表单验证规则
const passwordFormRules = {
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.newPassword) {
          callback(new Error('两次输入密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

// 加载用户列表
const loadUserList = async () => {
  tableLoading.value = true
  try {
    const response = await api.get('/admin/users')
    userList.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载用户列表失败')
  } finally {
    tableLoading.value = false
  }
}

// 打开添加用户对话框
const openAddDialog = () => {
  isEditMode.value = false
  userDialogVisible.value = true
}

// 打开编辑用户对话框
const openEditDialog = (user) => {
  isEditMode.value = true
  currentUser.value = user
  userForm.username = user.username
  userForm.email = user.email
  userForm.role = user.role
  userDialogVisible.value = true
}

// 重置用户表单
const resetUserForm = () => {
  userForm.username = ''
  userForm.email = ''
  userForm.password = ''
  userForm.role = ''
  currentUser.value = {}
  if (userFormRef.value) {
    userFormRef.value.resetFields()
  }
}

// 处理用户提交
const handleUserSubmit = async () => {
  if (!userFormRef.value) return
  
  try {
    await userFormRef.value.validate()
    userSubmitLoading.value = true
    
    if (isEditMode.value) {
      // 编辑用户
      await api.put(`/admin/users/${currentUser.value.id}`, {
        email: userForm.email,
        role: userForm.role
      })
      ElMessage.success('用户更新成功')
    } else {
      // 添加用户
      await api.post('/admin/users', {
        username: userForm.username,
        email: userForm.email,
        password: userForm.password,
        role: userForm.role
      })
      ElMessage.success('用户添加成功')
    }
    
    userDialogVisible.value = false
    loadUserList()
  } catch (error) {
    ElMessage.error(isEditMode.value ? '更新用户失败' : '添加用户失败')
  } finally {
    userSubmitLoading.value = false
  }
}

// 删除用户
const handleDeleteUser = async (user) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户 "${user.username}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    await api.delete(`/admin/users/${user.id}`)
    ElMessage.success('用户删除成功')
    loadUserList()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除用户失败')
    }
  }
}

// 打开重置密码对话框
const openResetPasswordDialog = (user) => {
  currentUser.value = user
  resetPasswordDialogVisible.value = true
}

// 重置密码表单
const resetPasswordForm = () => {
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  if (passwordFormRef.value) {
    passwordFormRef.value.resetFields()
  }
}

// 处理重置密码
const handleResetPassword = async () => {
  if (!passwordFormRef.value) return
  
  try {
    await passwordFormRef.value.validate()
    
    await ElMessageBox.confirm(
      `确定要重置用户 "${currentUser.value.username}" 的密码吗？`,
      '重置密码确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    resetPasswordLoading.value = true
    
    await api.post(`/admin/users/${currentUser.value.id}/reset-password`, {
      new_password: passwordForm.newPassword
    })
    
    ElMessage.success('密码重置成功')
    resetPasswordDialogVisible.value = false
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('重置密码失败')
    }
  } finally {
    resetPasswordLoading.value = false
  }
}

// 格式化日期时间
const formatDateTime = (dateString) => {
  if (!dateString) return '--'
  return new Date(dateString).toLocaleString('zh-CN')
}

// 组件挂载时加载数据
onMounted(() => {
  loadUserList()
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

.header-right {
  display: flex;
  align-items: center;
}
</style>
