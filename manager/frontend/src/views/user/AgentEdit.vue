<template>
  <div class="agent-config">
    <div class="config-header">
      <div class="header-left">
        <el-button 
          @click="$router.back()" 
          :icon="ArrowLeft" 
          circle 
          size="large"
        />
        <h1>智能体配置</h1>
      </div>
      <el-button type="primary" @click="handleSave" :loading="saving" size="large">
        保存配置
      </el-button>
    </div>

    <div class="config-content">
      <div class="config-form">
        <!-- 智能体名称 -->
        <div class="form-group">
          <label class="form-label">智能体名称</label>
          <el-input 
            v-model="form.name" 
            placeholder="请输入智能体名称" 
            size="large"
            :maxlength="50"
            show-word-limit
          />
        </div>

        <!-- 角色模板 -->
        <div class="form-group">
          <label class="form-label">角色模板</label>
          <div class="template-grid">
            <div 
              v-for="template in roleTemplates" 
              :key="template.id"
              class="template-card"
              :class="{ active: config.role_template === template.id }"
              @click="config.role_template = template.id"
            >
              <div class="template-icon">{{ template.icon }}</div>
              <div class="template-name">{{ template.name }}</div>
            </div>
          </div>
        </div>

        <!-- 助手名称 -->
        <div class="form-group">
          <label class="form-label">助手名称</label>
          <el-input 
            v-model="config.assistant_name" 
            placeholder="请输入助手名称" 
            size="large"
          />
        </div>

        <!-- 对话语言 -->
        <div class="form-group">
          <label class="form-label">对话语言</label>
          <el-select v-model="config.language" placeholder="请选择对话语言" size="large" style="width: 100%">
            <el-option label="中文" value="zh-CN" />
            <el-option label="English" value="en-US" />
            <el-option label="日本語" value="ja-JP" />
            <el-option label="한국어" value="ko-KR" />
          </el-select>
        </div>

        <!-- 角色音色 -->
        <div class="form-group">
          <label class="form-label">角色音色</label>
          <div class="voice-grid">
            <div 
              v-for="voice in voiceOptions" 
              :key="voice.id"
              class="voice-card"
              :class="{ active: config.voice_id === voice.id }"
              @click="config.voice_id = voice.id"
            >
              <div class="voice-avatar">{{ voice.avatar }}</div>
              <div class="voice-info">
                <div class="voice-name">{{ voice.name }}</div>
                <div class="voice-desc">{{ voice.description }}</div>
              </div>
              <el-button 
                :icon="VideoPlay" 
                circle 
                size="small" 
                @click.stop="playVoicePreview(voice.id)"
              />
            </div>
          </div>
        </div>

        <!-- 角色介绍 -->
        <div class="form-group">
          <label class="form-label">角色介绍</label>
          <el-input
            v-model="config.role_description"
            type="textarea"
            :rows="4"
            placeholder="请输入角色介绍，这将影响AI的回答风格和个性"
            :maxlength="500"
            show-word-limit
          />
        </div>

        <!-- 记忆体设置 -->
        <div class="form-group">
          <label class="form-label">记忆体设置</label>
          <div class="memory-settings">
            <div class="memory-item">
              <span>对话记忆长度</span>
              <el-slider 
                v-model="config.memory_length" 
                :min="5" 
                :max="50" 
                :step="5"
                show-stops
                show-input
                style="width: 200px;"
              />
            </div>
            <div class="memory-item">
              <span>启用长期记忆</span>
              <el-switch v-model="config.long_term_memory" />
            </div>
          </div>
        </div>

        <!-- LLM配置 -->
        <div class="form-group">
          <label class="form-label">LLM配置</label>
          <el-select 
            v-model="config.llm_config_id" 
            placeholder="请选择LLM配置" 
            size="large" 
            style="width: 100%"
            clearable
          >
            <el-option
              v-for="llmConfig in llmConfigs"
              :key="llmConfig.id"
              :label="llmConfig.name"
              :value="llmConfig.id"
            >
              <div class="config-option">
                <span class="config-name">{{ llmConfig.name }}</span>
                <span class="config-desc">{{ llmConfig.description || '暂无描述' }}</span>
              </div>
            </el-option>
          </el-select>
        </div>

        <!-- TTS配置 -->
        <div class="form-group">
          <label class="form-label">TTS配置</label>
          <el-select 
            v-model="config.tts_config_id" 
            placeholder="请选择TTS配置" 
            size="large" 
            style="width: 100%"
            clearable
          >
            <el-option
              v-for="ttsConfig in ttsConfigs"
              :key="ttsConfig.id"
              :label="ttsConfig.name"
              :value="ttsConfig.id"
            >
              <div class="config-option">
                <span class="config-name">{{ ttsConfig.name }}</span>
                <span class="config-desc">{{ ttsConfig.description || '暂无描述' }}</span>
              </div>
            </el-option>
          </el-select>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, VideoPlay } from '@element-plus/icons-vue'
import api from '@/utils/api'

const route = useRoute()
const router = useRouter()
const saving = ref(false)

// 表单数据
const form = reactive({
  name: ''
})

// 配置数据
const config = reactive({
  role_template: '',
  assistant_name: '',
  language: 'zh-CN',
  voice_id: '',
  role_description: '',
  memory_length: 20,
  long_term_memory: false,
  llm_config_id: null,
  tts_config_id: null
})

// 角色模板数据
const roleTemplates = ref([])

// 音色选项数据
const voiceOptions = ref([])

// LLM配置数据
const llmConfigs = ref([])

// TTS配置数据
const ttsConfigs = ref([])

// 加载LLM配置
const loadLlmConfigs = async () => {
  try {
    const response = await fetch('http://localhost:8080/api/user/llm-configs', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    })
    
    if (response.ok) {
      const data = await response.json()
      llmConfigs.value = data.data || []
    }
  } catch (error) {
    console.error('加载LLM配置失败:', error)
  }
}

// 加载TTS配置
const loadTtsConfigs = async () => {
  try {
    const response = await fetch('http://localhost:8080/api/user/tts-configs', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    })
    
    if (response.ok) {
      const data = await response.json()
      ttsConfigs.value = data.data || []
    }
  } catch (error) {
    console.error('加载TTS配置失败:', error)
  }
}



// 加载智能体数据
const loadAgent = async () => {
  try {
    const response = await fetch(`http://localhost:8080/api/user/agents/${route.params.id}`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    })
    
    if (response.ok) {
      const data = await response.json()
      form.name = data.data.name
      
      if (data.data.config) {
        try {
          const configData = JSON.parse(data.data.config)
          Object.assign(config, configData)
        } catch (e) {
          console.error('解析配置失败:', e)
        }
      }
    }
  } catch (error) {
    console.error('加载智能体失败:', error)
    ElMessage.error('加载智能体失败')
  }
}

// 加载角色模板
const loadRoleTemplates = async () => {
  try {
    const response = await fetch('http://localhost:8080/api/user/role-templates', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    })
    
    if (response.ok) {
      const data = await response.json()
      roleTemplates.value = data.data || []
    }
  } catch (error) {
    console.error('加载角色模板失败:', error)
  }
}

// 加载音色选项
const loadVoiceOptions = async () => {
  try {
    const response = await fetch('http://localhost:8080/api/user/voice-options', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    })
    
    if (response.ok) {
      const data = await response.json()
      voiceOptions.value = data.data || []
    }
  } catch (error) {
    console.error('加载音色选项失败:', error)
  }
}

// 保存智能体
const handleSave = async () => {
  if (!form.name.trim()) {
    ElMessage.error('请输入智能体名称')
    return
  }
  
  try {
    saving.value = true
    
    const agentData = {
      name: form.name,
      config: JSON.stringify(config)
    }
    
    const response = await fetch(`http://localhost:8080/api/user/agents/${route.params.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify(agentData)
    })
    
    if (response.ok) {
      const data = await response.json()
      if (data.success) {
        ElMessage.success('保存成功')
        router.push('/agents')
      } else {
        ElMessage.error('保存失败')
      }
    } else {
      const errorData = await response.json()
      ElMessage.error(errorData.error || '保存失败')
    }
  } catch (error) {
    console.error('保存失败:', error)
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

// 播放音色预览
const playVoicePreview = (voiceId) => {
  // 这里可以添加音色预览播放逻辑
  ElMessage.info(`播放音色预览: ${voiceId}`)
}

onMounted(async () => {
  // 并行加载数据
  await Promise.all([
    loadRoleTemplates(),
    loadVoiceOptions(),
    loadLlmConfigs(),
    loadTtsConfigs()
  ])
  
  if (route.params.id) {
    await loadAgent()
  }
})
</script>

<style scoped>
.agent-config {
  min-height: 100vh;
  background: #f8fafc;
  padding: 24px;
}

.config-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
  background: white;
  padding: 20px 24px;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-left h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: #1f2937;
}

.config-content {
  max-width: 800px;
  margin: 0 auto;
}

.config-form {
  background: white;
  border-radius: 12px;
  padding: 32px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.form-group {
  margin-bottom: 32px;
}

.form-group:last-child {
  margin-bottom: 0;
}

.form-label {
  display: block;
  font-size: 16px;
  font-weight: 600;
  color: #374151;
  margin-bottom: 12px;
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
}

.template-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 16px;
  border: 2px solid #e5e7eb;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: #fafafa;
}

.template-card:hover {
  border-color: #3b82f6;
  background: #f0f9ff;
}

.template-card.active {
  border-color: #3b82f6;
  background: #eff6ff;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.template-icon {
  font-size: 32px;
  margin-bottom: 8px;
}

.template-name {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
  text-align: center;
}

.voice-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.voice-card {
  display: flex;
  align-items: center;
  padding: 16px;
  border: 2px solid #e5e7eb;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: #fafafa;
}

.voice-card:hover {
  border-color: #3b82f6;
  background: #f0f9ff;
}

.voice-card.active {
  border-color: #3b82f6;
  background: #eff6ff;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.voice-avatar {
  font-size: 24px;
  margin-right: 16px;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: white;
  border-radius: 50%;
  border: 1px solid #e5e7eb;
}

.voice-info {
  flex: 1;
}

.voice-name {
  font-size: 16px;
  font-weight: 600;
  color: #374151;
  margin-bottom: 4px;
}

.voice-desc {
  font-size: 14px;
  color: #6b7280;
}

.memory-settings {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.memory-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  background: #f9fafb;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
}

.memory-item span {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.config-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.config-name {
  font-weight: 500;
  color: #374151;
}

.config-desc {
  font-size: 12px;
  color: #6b7280;
}

@media (max-width: 768px) {
  .agent-config {
    padding: 16px;
  }
  
  .config-header {
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }
  
  .header-left {
    justify-content: center;
  }
  
  .config-form {
    padding: 24px 16px;
  }
  
  .template-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .memory-item {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }
}
</style>