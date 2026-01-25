import { describe, it, expect } from 'vitest'

describe('Type definitions', () => {
  it('User type structure is correct', () => {
    const user = {
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
    expect(user.status).toBe('active')
  })

  it('Environment type structure is correct', () => {
    const env = {
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
    expect(env.resources.memory).toBe('512Mi')
  })

  it('APIKey type structure is correct', () => {
    const apiKey = {
      id: 'key-1',
      key_prefix: 'ak_live_',
      description: 'Test key',
      created_at: '2024-01-01T00:00:00Z',
    }

    expect(apiKey.id).toBe('key-1')
    expect(apiKey.key_prefix).toBe('ak_live_')
    expect(apiKey.description).toBe('Test key')
  })

  it('Metric type structure is correct', () => {
    const metric = {
      id: 'metric-1',
      metric_type: 'running_sandboxes',
      value: 42,
      timestamp: '2024-01-01T00:00:00Z',
    }

    expect(metric.id).toBe('metric-1')
    expect(metric.metric_type).toBe('running_sandboxes')
    expect(metric.value).toBe(42)
  })
})
