export interface User {
  id: string
  username: string
  email?: string
  role: string
  status: string
  created_at: string
  updated_at: string
  last_login?: string
}

export interface Environment {
  id: string
  name: string
  status: 'pending' | 'running' | 'terminated' | 'failed'
  image: string
  resources: {
    cpu: string
    memory: string
    storage: string
  }
  created_at: string
  started_at?: string
  terminated_at?: string
}

export interface APIKey {
  id: string
  key_prefix: string
  description?: string
  created_at: string
  last_used?: string
  expires_at?: string
  revoked_at?: string
}

export interface Metric {
  id: string
  environment_id?: string
  metric_type: string
  value: number
  timestamp: string
}

export interface LogEntry {
  message: string
  timestamp?: string
}

export interface CreateEnvironmentData {
  name: string
  image: string
  resources: {
    cpu: string
    memory: string
    storage: string
  }
}

export interface CreateUserData {
  username: string
  email?: string
  password: string
  role: string
  status: string
}

export interface CreateAPIKeyData {
  description?: string
  expiresIn?: number
}
