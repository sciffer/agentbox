import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './authStore'

describe('authStore', () => {
  beforeEach(() => {
    // Reset the store before each test
    useAuthStore.setState({
      token: null,
      user: null,
      isAuthenticated: false,
    })
  })

  it('should start with unauthenticated state', () => {
    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(false)
    expect(state.token).toBeNull()
    expect(state.user).toBeNull()
  })

  it('should set auth correctly', () => {
    const mockUser = {
      id: '1',
      username: 'testuser',
      email: 'test@example.com',
      role: 'admin',
      status: 'active',
    }
    const mockToken = 'test-jwt-token'

    useAuthStore.getState().setAuth(mockToken, mockUser)

    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(true)
    expect(state.token).toBe(mockToken)
    expect(state.user).toEqual(mockUser)
  })

  it('should clear auth correctly', () => {
    // First set some auth
    useAuthStore.getState().setAuth('token', {
      id: '1',
      username: 'user',
      role: 'user',
      status: 'active',
    })

    // Then clear it
    useAuthStore.getState().clearAuth()

    const state = useAuthStore.getState()
    expect(state.isAuthenticated).toBe(false)
    expect(state.token).toBeNull()
    expect(state.user).toBeNull()
  })
})
