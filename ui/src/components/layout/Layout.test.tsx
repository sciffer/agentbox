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
    // Use getAllByText since "AgentBox" appears multiple times (drawer + appbar)
    const titles = screen.getAllByText(/AgentBox/i)
    expect(titles.length).toBeGreaterThan(0)
  })

  it('renders navigation items', () => {
    render(<Layout />)
    // Navigation items appear in both temporary and permanent drawers
    // Use getAllByText to handle multiple instances
    const dashboardItems = screen.getAllByText('Dashboard')
    expect(dashboardItems.length).toBeGreaterThan(0)
    
    const environmentItems = screen.getAllByText('Environments')
    expect(environmentItems.length).toBeGreaterThan(0)
    
    const usersItems = screen.getAllByText('Users')
    expect(usersItems.length).toBeGreaterThan(0)
  })

  it('renders the outlet for child routes', () => {
    render(<Layout />)
    expect(screen.getByTestId('outlet')).toBeInTheDocument()
  })

  it('displays user avatar with first letter of username', () => {
    render(<Layout />)
    // The avatar should show 'T' for 'testuser'
    expect(screen.getByText('T')).toBeInTheDocument()
  })
})
