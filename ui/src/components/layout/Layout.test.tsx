import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { render } from '../../test/test-utils'
import Layout from './Layout'
import { useAuthStore } from '../../store/authStore'

// Mock react-router-dom
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    useLocation: () => ({ pathname: '/' }),
    Outlet: () => <div data-testid="outlet">Outlet Content</div>,
  }
})

describe('Layout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Set up a mock authenticated user
    useAuthStore.setState({
      token: 'test-token',
      user: {
        id: '1',
        username: 'testuser',
        email: 'test@example.com',
        role: 'admin',
        status: 'active',
      },
      isAuthenticated: true,
    })
  })

  it('renders the app title', () => {
    render(<Layout />)
    expect(screen.getByText('AgentBox')).toBeInTheDocument()
  })

  it('renders navigation items', () => {
    render(<Layout />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Environments')).toBeInTheDocument()
    expect(screen.getByText('Users')).toBeInTheDocument()
  })

  it('renders the outlet for child routes', () => {
    render(<Layout />)
    expect(screen.getByTestId('outlet')).toBeInTheDocument()
  })

  it('displays user avatar', () => {
    render(<Layout />)
    // The avatar should show 'T' for 'testuser'
    expect(screen.getByText('T')).toBeInTheDocument()
  })
})
