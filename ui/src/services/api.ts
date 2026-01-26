import axios, { AxiosError } from 'axios'
import { useAuthStore } from '../store/authStore'
import { 
  CreateEnvironmentData, 
  CreateUserData, 
  UpdateUserData, 
  GrantPermissionData,
  CreateAPIKeyData,
  APIKeyPermission,
  SubmitExecutionData,
  Execution,
  ExecutionListResponse
} from '../types'

// Runtime config from window.AGENTBOX_CONFIG (set by Docker entrypoint)
// Falls back to build-time env var, then to relative path for API proxy
declare global {
  interface Window {
    AGENTBOX_CONFIG?: {
      API_URL?: string
      WS_URL?: string
      GOOGLE_OAUTH_ENABLED?: string
    }
  }
}

function getApiUrl(): string {
  // First try runtime config (Docker/Kubernetes)
  if (typeof window !== 'undefined' && window.AGENTBOX_CONFIG?.API_URL) {
    return window.AGENTBOX_CONFIG.API_URL
  }
  // Then try build-time env var (development)
  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL
  }
  // Default to relative path (nginx proxy handles /api)
  return '/api/v1'
}

const API_URL = getApiUrl()

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
  exec: async (id: string, command: string[], timeout?: number) => {
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

// Executions API (async isolated pod execution)
export const executionsAPI = {
  // Submit a new execution (returns immediately with execution ID)
  submit: async (environmentId: string, data: SubmitExecutionData): Promise<Execution> => {
    const response = await apiClient.post(`/environments/${environmentId}/run`, data)
    return response.data
  },
  // Get execution status and results
  get: async (id: string): Promise<Execution> => {
    const response = await apiClient.get(`/executions/${id}`)
    return response.data
  },
  // List executions for an environment
  list: async (environmentId: string, params?: { limit?: number }): Promise<ExecutionListResponse> => {
    const response = await apiClient.get(`/environments/${environmentId}/executions`, { params })
    return response.data
  },
  // Cancel an execution
  cancel: async (id: string): Promise<void> => {
    await apiClient.delete(`/executions/${id}`)
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
  update: async (id: string, data: UpdateUserData) => {
    const response = await apiClient.put(`/users/${id}`, data)
    return response.data
  },
  delete: async (id: string) => {
    await apiClient.delete(`/users/${id}`)
  },
  // Permission methods
  listPermissions: async (userId: string) => {
    const response = await apiClient.get(`/users/${userId}/permissions`)
    return response.data
  },
  grantPermission: async (userId: string, data: GrantPermissionData) => {
    const response = await apiClient.post(`/users/${userId}/permissions`, data)
    return response.data
  },
  updatePermission: async (userId: string, envId: string, permission: string) => {
    const response = await apiClient.put(`/users/${userId}/permissions/${envId}`, { permission })
    return response.data
  },
  revokePermission: async (userId: string, envId: string) => {
    await apiClient.delete(`/users/${userId}/permissions/${envId}`)
  },
}

// API Keys API
export const apiKeysAPI = {
  list: async () => {
    const response = await apiClient.get('/api-keys')
    return response.data
  },
  create: async (data: CreateAPIKeyData) => {
    const response = await apiClient.post('/api-keys', {
      description: data.description,
      expires_in: data.expires_in,
      permissions: data.permissions?.map((p: APIKeyPermission) => ({
        environment_id: p.environment_id,
        permission: p.permission,
      })),
    })
    return response.data
  },
  revoke: async (id: string) => {
    await apiClient.delete(`/api-keys/${id}`)
  },
  listPermissions: async (id: string) => {
    const response = await apiClient.get(`/api-keys/${id}/permissions`)
    return response.data
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
