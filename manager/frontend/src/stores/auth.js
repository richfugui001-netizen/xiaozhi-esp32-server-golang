import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../utils/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token'))
  const user = ref(JSON.parse(localStorage.getItem('user') || 'null'))

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  const login = async (credentials) => {
    try {
      const response = await api.post('/login', credentials)
      const { token: newToken, user: userData } = response.data
      
      token.value = newToken
      user.value = userData
      
      localStorage.setItem('token', newToken)
      localStorage.setItem('user', JSON.stringify(userData))
      
      return { success: true, user: userData }
    } catch (error) {
      return { 
        success: false, 
        message: error.response?.data?.error || '登录失败' 
      }
    }
  }

  const register = async (userData) => {
    try {
      await api.post('/register', userData)
      return { success: true }
    } catch (error) {
      return { 
        success: false, 
        message: error.response?.data?.error || '注册失败' 
      }
    }
  }

  const logout = () => {
    token.value = null
    user.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  const getProfile = async () => {
    try {
      const response = await api.get('/profile')
      user.value = response.data.user
      localStorage.setItem('user', JSON.stringify(response.data.user))
    } catch (error) {
      logout()
    }
  }

  return {
    token,
    user,
    isAuthenticated,
    isAdmin,
    login,
    register,
    logout,
    getProfile
  }
})