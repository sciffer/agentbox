import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import { apiClient, authAPI, environmentsAPI, usersAPI, apiKeysAPI } from './api'

// Mock axios
vi.mock('axios', () => {
  const mockAxios = {
    create: vi.fn(() => mockAxios),
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
  }
  return { default: mockAxios }
})

describe('API Service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('authAPI', () => {
    it('login should call POST /auth/login', async () => {
      const mockResponse = { data: { token: 'jwt-token', user: { id: '1' } } }
      vi.mocked(axios.create().post).mockResolvedValue(mockResponse)

      const result = await authAPI.login('admin', 'password')

      expect(axios.create().post).toHaveBeenCalledWith('/auth/login', {
        username: 'admin',
        password: 'password',
      })
      expect(result).toEqual(mockResponse.data)
    })

    it('logout should call POST /auth/logout', async () => {
      vi.mocked(axios.create().post).mockResolvedValue({ data: {} })

      await authAPI.logout()

      expect(axios.create().post).toHaveBeenCalledWith('/auth/logout')
    })

    it('getMe should call GET /auth/me', async () => {
      const mockUser = { id: '1', username: 'admin' }
      vi.mocked(axios.create().get).mockResolvedValue({ data: mockUser })

      const result = await authAPI.getMe()

      expect(axios.create().get).toHaveBeenCalledWith('/auth/me')
      expect(result).toEqual(mockUser)
    })
  })

  describe('environmentsAPI', () => {
    it('list should call GET /environments', async () => {
      const mockEnvs = { environments: [], total: 0 }
      vi.mocked(axios.create().get).mockResolvedValue({ data: mockEnvs })

      const result = await environmentsAPI.list()

      expect(axios.create().get).toHaveBeenCalledWith('/environments', { params: undefined })
      expect(result).toEqual(mockEnvs)
    })

    it('get should call GET /environments/:id', async () => {
      const mockEnv = { id: 'env-1', name: 'test' }
      vi.mocked(axios.create().get).mockResolvedValue({ data: mockEnv })

      const result = await environmentsAPI.get('env-1')

      expect(axios.create().get).toHaveBeenCalledWith('/environments/env-1')
      expect(result).toEqual(mockEnv)
    })

    it('create should call POST /environments', async () => {
      const mockEnv = { id: 'env-1', name: 'new-env' }
      vi.mocked(axios.create().post).mockResolvedValue({ data: mockEnv })

      const result = await environmentsAPI.create({ name: 'new-env', image: 'python:3.11' })

      expect(axios.create().post).toHaveBeenCalledWith('/environments', {
        name: 'new-env',
        image: 'python:3.11',
      })
      expect(result).toEqual(mockEnv)
    })

    it('delete should call DELETE /environments/:id', async () => {
      vi.mocked(axios.create().delete).mockResolvedValue({ data: {} })

      await environmentsAPI.delete('env-1')

      expect(axios.create().delete).toHaveBeenCalledWith('/environments/env-1', { params: { force: undefined } })
    })
  })

  describe('usersAPI', () => {
    it('list should call GET /users', async () => {
      const mockUsers = { users: [], total: 0 }
      vi.mocked(axios.create().get).mockResolvedValue({ data: mockUsers })

      const result = await usersAPI.list()

      expect(axios.create().get).toHaveBeenCalledWith('/users', { params: undefined })
      expect(result).toEqual(mockUsers)
    })

    it('create should call POST /users', async () => {
      const mockUser = { id: '1', username: 'newuser' }
      vi.mocked(axios.create().post).mockResolvedValue({ data: mockUser })

      const result = await usersAPI.create({ username: 'newuser', password: 'pass' })

      expect(axios.create().post).toHaveBeenCalledWith('/users', {
        username: 'newuser',
        password: 'pass',
      })
      expect(result).toEqual(mockUser)
    })
  })

  describe('apiKeysAPI', () => {
    it('list should call GET /api-keys', async () => {
      const mockKeys = { api_keys: [] }
      vi.mocked(axios.create().get).mockResolvedValue({ data: mockKeys })

      const result = await apiKeysAPI.list()

      expect(axios.create().get).toHaveBeenCalledWith('/api-keys')
      expect(result).toEqual(mockKeys)
    })

    it('create should call POST /api-keys', async () => {
      const mockKey = { id: '1', key: 'ak_...' }
      vi.mocked(axios.create().post).mockResolvedValue({ data: mockKey })

      const result = await apiKeysAPI.create('Test key')

      expect(axios.create().post).toHaveBeenCalledWith('/api-keys', {
        description: 'Test key',
        expires_in: undefined,
      })
      expect(result).toEqual(mockKey)
    })

    it('revoke should call DELETE /api-keys/:id', async () => {
      vi.mocked(axios.create().delete).mockResolvedValue({ data: {} })

      await apiKeysAPI.revoke('key-1')

      expect(axios.create().delete).toHaveBeenCalledWith('/api-keys/key-1')
    })
  })
})
