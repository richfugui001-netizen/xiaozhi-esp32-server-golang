<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>全局角色管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showDialog = true">
          <el-icon><Plus /></el-icon>
          添加角色
        </el-button>
      </div>
    </div>

    <el-table :data="roles" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="角色名称" />
      <el-table-column prop="description" label="描述" />
      <el-table-column prop="is_default" label="默认角色" width="100">
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
          <el-button size="small" @click="editRole(scope.row)">编辑</el-button>
          <el-button
            size="small"
            type="danger"
            @click="deleteRole(scope.row.id)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑角色弹窗 -->
    <el-dialog
      v-model="showDialog"
      :title="editingRole ? '编辑全局角色' : '添加全局角色'"
      width="600px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入角色名称" />
        </el-form-item>
        
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入角色描述"
          />
        </el-form-item>
        
        <el-form-item label="是否默认" prop="is_default">
          <el-switch v-model="form.is_default" />
        </el-form-item>
        
        <el-form-item label="系统提示词" prop="prompt">
          <el-input
            v-model="form.prompt"
            type="textarea"
            :rows="8"
            placeholder="请输入系统提示词"
          />
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
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

const roles = ref([])
const loading = ref(false)
const saving = ref(false)
const showDialog = ref(false)
const editingRole = ref(null)
const formRef = ref()

const form = reactive({
  name: '',
  description: '',
  is_default: false,
  prompt: ''
})

const rules = {
  name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
  prompt: [{ required: true, message: '请输入系统提示词', trigger: 'blur' }]
}

const loadRoles = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/global-roles')
    roles.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载角色失败')
  } finally {
    loading.value = false
  }
}

const editRole = (role) => {
  editingRole.value = role
  Object.assign(form, {
    name: role.name,
    description: role.description,
    is_default: role.is_default,
    prompt: role.prompt
  })
  showDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        if (editingRole.value) {
          await api.put(`/admin/global-roles/${editingRole.value.id}`, form)
          ElMessage.success('更新成功')
        } else {
          await api.post('/admin/global-roles', form)
          ElMessage.success('添加成功')
        }
        
        showDialog.value = false
        loadRoles()
      } catch (error) {
        ElMessage.error('保存失败')
      } finally {
        saving.value = false
      }
    }
  })
}

const deleteRole = async (id) => {
  try {
    await ElMessageBox.confirm('确定要删除这个角色吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.delete(`/admin/global-roles/${id}`)
    ElMessage.success('删除成功')
    loadRoles()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const resetForm = () => {
  editingRole.value = null
  Object.assign(form, {
    name: '',
    description: '',
    is_default: false,
    prompt: ''
  })
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
  loadRoles()
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