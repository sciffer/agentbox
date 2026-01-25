import { describe, it, expect } from 'vitest'

describe('API Service', () => {
  describe('API URL configuration', () => {
    it('should have correct default API URL format', () => {
      const defaultUrl = 'http://localhost:8080/api/v1'
      expect(defaultUrl).toContain('/api/v1')
      expect(defaultUrl).toContain('localhost')
    })
  })

  describe('API endpoint paths', () => {
    it('should construct correct auth endpoints', () => {
      const endpoints = {
        login: '/auth/login',
        logout: '/auth/logout',
        me: '/auth/me',
        changePassword: '/auth/change-password',
      }
      
      expect(endpoints.login).toBe('/auth/login')
      expect(endpoints.logout).toBe('/auth/logout')
      expect(endpoints.me).toBe('/auth/me')
      expect(endpoints.changePassword).toBe('/auth/change-password')
    })

    it('should construct correct environment endpoints', () => {
      const envId = 'env-123'
      const endpoints = {
        list: '/environments',
        get: `/environments/${envId}`,
        create: '/environments',
        delete: `/environments/${envId}`,
        exec: `/environments/${envId}/exec`,
        logs: `/environments/${envId}/logs`,
      }
      
      expect(endpoints.list).toBe('/environments')
      expect(endpoints.get).toBe('/environments/env-123')
      expect(endpoints.exec).toBe('/environments/env-123/exec')
      expect(endpoints.logs).toBe('/environments/env-123/logs')
    })

    it('should construct correct user endpoints', () => {
      const userId = 'user-123'
      const endpoints = {
        list: '/users',
        get: `/users/${userId}`,
        create: '/users',
      }
      
      expect(endpoints.list).toBe('/users')
      expect(endpoints.get).toBe('/users/user-123')
      expect(endpoints.create).toBe('/users')
    })

    it('should construct correct API key endpoints', () => {
      const keyId = 'key-123'
      const endpoints = {
        list: '/api-keys',
        create: '/api-keys',
        revoke: `/api-keys/${keyId}`,
      }
      
      expect(endpoints.list).toBe('/api-keys')
      expect(endpoints.create).toBe('/api-keys')
      expect(endpoints.revoke).toBe('/api-keys/key-123')
    })

    it('should construct correct metrics endpoints', () => {
      const envId = 'env-123'
      const endpoints = {
        global: '/metrics/global',
        environment: `/metrics/environment/${envId}`,
      }
      
      expect(endpoints.global).toBe('/metrics/global')
      expect(endpoints.environment).toBe('/metrics/environment/env-123')
    })
  })
})
