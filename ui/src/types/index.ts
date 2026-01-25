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

export interface Toleration {
  key?: string
  operator?: 'Exists' | 'Equal'
  value?: string
  effect?: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute'
  tolerationSeconds?: number
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
  node_selector?: Record<string, string>
  tolerations?: Toleration[]
}

export interface EnvironmentPermission {
  id: string
  user_id: string
  environment_id: string
  permission: 'viewer' | 'editor' | 'owner'
  granted_by?: string
  granted_at: string
}

export interface APIKeyPermission {
  id?: string
  api_key_id?: string
  environment_id: string
  permission: 'viewer' | 'editor' | 'owner'
  created_at?: string
}

export interface APIKey {
  id: string
  key?: string // Only returned on creation
  key_prefix: string
  description?: string
  created_at: string
  last_used?: string
  expires_at?: string
  revoked_at?: string
  permissions?: APIKeyPermission[]
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
  labels?: Record<string, string>
  env?: Record<string, string>
  command?: string[]
  timeout?: number
  node_selector?: Record<string, string>
  tolerations?: Toleration[]
}

export interface CreateUserData {
  username: string
  email?: string
  password: string
  role: string
  status: string
}

export interface UpdateUserData {
  username?: string
  email?: string
  password?: string
  role?: string
  status?: string
}

export interface GrantPermissionData {
  environment_id: string
  permission: 'viewer' | 'editor' | 'owner'
}

export interface CreateAPIKeyData {
  description?: string
  expires_in?: number
  permissions?: APIKeyPermission[]
}
