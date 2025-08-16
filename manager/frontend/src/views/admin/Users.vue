<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>用户管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="addUser">
          <el-icon><Plus /></el-icon>
          添加用户
        </el-button>
      </div>
    </div>

    <el-table :data="users" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="email" label="邮箱" />
      <el-table-column prop="role" label="角色" width="100">
        <template #default="scope">
          <el-tag :type="scope.row.role === 'admin' ? 'danger' : 'primary'">
            {{ scope.row.role === 'admin' ? '管理员' : '普通用户' }}
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
          <el-button size="small" @click="editUser(scope.row)">编辑</el-button>
          <el-button
            size="small"
            type="danger"
            @click="deleteUser(scope.row.id)"
            :disabled="scope.row.role === 'admin'"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 用户弹窗 -->
    <el-dialog
      v-model="showDialog"
      :title="editingUser ? '编辑用户' : '添加用户'"
      width="500px"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" :disabled="!!editingUser" placeholder="请输入用户名" />
        </el-form-item>
        
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="请输入邮箱" />
        </el-form-item>
        
        <el-form-item v-if="!editingUser" label="密码" prop="password">
          <el-input v-model="form.password" type="password" placeholder="请输入密码" show-password />
        </el-form-item>
        
        <el-form-item label="角色" prop="role">
          <el-select v-model="form.role" placeholder="请选择角色" style="width: 100%">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
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
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const showDialog = ref(false)
const editingUser = ref(null)
const formRef = ref()

const form = reactive({
  username: '',
  email: '',
  password: '',
  role: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' }
  ],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  role: [{ required: true, message: '请选择角色', trigger: 'change' }]
}

const loadUsers = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/users')
    users.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载用户失败')
  } finally {
    loading.value = false
  }
}

const addUser = () => {
  editingUser.value = null
  resetForm()
  showDialog.value = true
}

const editUser = (user) => {
  editingUser.value = user
  Object.assign(form, {
    username: user.username,
    email: user.email,
    password: '',
    role: user.role
  })
  showDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        if (editingUser.value) {
          // 编辑用户
          await api.put(`/admin/users/${editingUser.value.id}`, {
            email: form.email,
            role: form.role
          })
          ElMessage.success('更新成功')
        } else {
          // 添加用户
          await api.post('/admin/users', {
            username: form.username,
            email: form.email,
            password: form.password,
            role: form.role
          })
          ElMessage.success('添加成功')
        }
        
        showDialog.value = false
        resetForm()
        loadUsers()
      } catch (error) {
        ElMessage.error(editingUser.value ? '保存失败' : '添加失败')
      } finally {
        saving.value = false
      }
    }
  })
}

const deleteUser = async (id) => {
  try {
    await ElMessageBox.confirm('确定要删除这个用户吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.delete(`/admin/users/${id}`)
    ElMessage.success('删除成功')
    loadUsers()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const resetForm = () => {
  editingUser.value = null
  Object.assign(form, {
    username: '',
    email: '',
    password: '',
    role: ''
  })
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadUsers()
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