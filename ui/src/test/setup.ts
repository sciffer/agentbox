import '@testing-library/jest-dom'
import { afterEach, vi, beforeEach } from 'vitest'
import { cleanup } from '@testing-library/react'

// Suppress React Router future flag warnings in tests
const originalWarn = console.warn
const originalError = console.error

beforeEach(() => {
  console.warn = (...args: unknown[]) => {
    const message = typeof args[0] === 'string' ? args[0] : ''
    // Suppress React Router v7 future flag warnings
    if (message.includes('React Router Future Flag Warning')) {
      return
    }
    originalWarn.apply(console, args)
  }
  
  console.error = (...args: unknown[]) => {
    const message = typeof args[0] === 'string' ? args[0] : ''
    // Suppress act() warnings for known MUI animation issues
    if (message.includes('was not wrapped in act')) {
      return
    }
    originalError.apply(console, args)
  }
})

afterEach(() => {
  console.warn = originalWarn
  console.error = originalError
})

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
    get length() {
      return Object.keys(store).length
    },
    key: (index: number) => Object.keys(store)[index] || null,
  }
})()
vi.stubGlobal('localStorage', localStorageMock)

// Clear localStorage before each test
beforeEach(() => {
  localStorage.clear()
})

// Cleanup after each test
afterEach(() => {
  cleanup()
})

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

// Mock ResizeObserver as a class
class ResizeObserverMock {
  observe = vi.fn()
  unobserve = vi.fn()
  disconnect = vi.fn()
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock)

// Mock IntersectionObserver as a class
class IntersectionObserverMock {
  observe = vi.fn()
  unobserve = vi.fn()
  disconnect = vi.fn()
}
vi.stubGlobal('IntersectionObserver', IntersectionObserverMock)
