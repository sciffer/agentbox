import { describe, it, expect } from 'vitest'
import type { User, Environment, APIKey, Metric } from './index'

describe('Type definitions', () => {
  it('User type should have required fields', () => {
    const user: User = {
      id: '1',
      username: 'testuser',
      email: 'test@example.com',
      role: 'admin',
      status: 'active',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }

    expect(user.id).toBe('1')
    expect(user.username).toBe('testuser')
    expect(user.role).toBe('admin')
  })

  it('Environment type should have required fields', () => {
    const env: Environment = {
      id: 'env-1',
      name: 'test-env',
      status: 'running',
      image: 'python:3.11',
      resources: {
        cpu: '500m',
        memory: '512Mi',
        storage: '1Gi',
      },
      created_at: '2024-01-01T00:00:00Z',
    }

    expect(env.id).toBe('env-1')
    expect(env.status).toBe('running')
    expect(env.resources.cpu).toBe('500m')
  })

  it('APIKey type should have required fields', () => {
    const apiKey: APIKey = {
      id: 'key-1',
      key_prefix: 'ak_live_',
      description: 'Test key',
      created_at: '2024-01-01T00:00:00Z',
    }

    expect(apiKey.id).toBe('key-1')
    expect(apiKey.key_prefix).toBe('ak_live_')
  })

  it('Metric type should have required fields', () => {
    const metric: Metric = {
      id: 'metric-1',
      metric_type: 'running_sandboxes',
      value: 42,
      timestamp: '2024-01-01T00:00:00Z',
    }

    expect(metric.id).toBe('metric-1')
    expect(metric.value).toBe(42)
  })
})
