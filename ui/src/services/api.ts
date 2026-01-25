import axios, { AxiosError } from 'axios'
import { useAuthStore } from '../store/authStore'
import { CreateEnvironmentData, CreateUserData } from '../types'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1'

export const apiClient = axios.create({
  baseURL: API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor to add auth token
apiClient.interceptors.request.use(
  (config) => {
    const token = useAuthStore.getState().token
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor to handle auth errors
apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().clearAuth()
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// Auth API
export const authAPI = {
  login: async (username: string, password: string) => {
    const response = await apiClient.post('/auth/login', { username, password })
    return response.data
  },
  logout: async () => {
    await apiClient.post('/auth/logout')
  },
  getMe: async () => {
    const response = await apiClient.get('/auth/me')
    return response.data
  },
  changePassword: async (currentPassword: string, newPassword: string) => {
    await apiClient.post('/auth/change-password', {
      current_password: currentPassword,
      new_password: newPassword,
    })
  },
}

// Environments API
export const environmentsAPI = {
  list: async (params?: { status?: string; limit?: number; offset?: number }) => {
    const response = await apiClient.get('/environments', { params })
    return response.data
  },
  get: async (id: string) => {
    const response = await apiClient.get(`/environments/${id}`)
    return response.data
  },
  create: async (data: CreateEnvironmentData) => {
    const response = await apiClient.post('/environments', data)
    return response.data
  },
  delete: async (id: string, force?: boolean) => {
    await apiClient.delete(`/environments/${id}`, { params: { force } })
  },
  exec: async (id: string, command: string, timeout?: number) => {
    const response = await apiClient.post(`/environments/${id}/exec`, {
      command,
      timeout,
    })
    return response.data
  },
  getLogs: async (id: string, params?: { tail?: number; follow?: boolean; timestamps?: boolean }) => {
    const response = await apiClient.get(`/environments/${id}/logs`, { params })
    return response.data
  },
}

// Users API
export const usersAPI = {
  list: async (params?: { limit?: number; offset?: number }) => {
    const response = await apiClient.get('/users', { params })
    return response.data
  },
  get: async (id: string) => {
    const response = await apiClient.get(`/users/${id}`)
    return response.data
  },
  create: async (data: CreateUserData) => {
    const response = await apiClient.post('/users', data)
    return response.data
  },
}

// API Keys API
export const apiKeysAPI = {
  list: async () => {
    const response = await apiClient.get('/api-keys')
    return response.data
  },
  create: async (description?: string, expiresIn?: number) => {
    const response = await apiClient.post('/api-keys', {
      description,
      expires_in: expiresIn,
    })
    return response.data
  },
  revoke: async (id: string) => {
    await apiClient.delete(`/api-keys/${id}`)
  },
}

// Metrics API
export const metricsAPI = {
  getGlobal: async (params?: { type?: string; start?: string; end?: string }) => {
    const response = await apiClient.get('/metrics/global', { params })
    return response.data
  },
  getEnvironment: async (id: string, params?: { type?: string; start?: string; end?: string }) => {
    const response = await apiClient.get(`/metrics/environment/${id}`, { params })
    return response.data
  },
}
