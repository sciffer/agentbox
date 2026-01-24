import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { render } from '../../test/test-utils'
import Layout from './Layout'
import { useAuthStore } from '../../store/authStore'

// Mock useNavigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
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

  it('renders the layout with navigation', () => {
    render(<Layout />)

    expect(screen.getByText('AgentBox')).toBeInTheDocument()
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Environments')).toBeInTheDocument()
    expect(screen.getByText('Users')).toBeInTheDocument()
    expect(screen.getByText('API Keys')).toBeInTheDocument()
    expect(screen.getByText('Settings')).toBeInTheDocument()
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

  it('navigates to dashboard when clicking Dashboard menu item', async () => {
    const user = userEvent.setup()
    render(<Layout />)

    await user.click(screen.getByText('Dashboard'))

    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('navigates to environments when clicking Environments menu item', async () => {
    const user = userEvent.setup()
    render(<Layout />)

    await user.click(screen.getByText('Environments'))

    expect(mockNavigate).toHaveBeenCalledWith('/environments')
  })

  it('logs out when clicking logout menu item', async () => {
    const user = userEvent.setup()
    render(<Layout />)

    // Click on the avatar to open the menu
    const avatar = screen.getByText('T')
    await user.click(avatar)

    // Click on logout
    const logoutButton = screen.getByText('Logout')
    await user.click(logoutButton)

    expect(mockNavigate).toHaveBeenCalledWith('/login')
    expect(useAuthStore.getState().isAuthenticated).toBe(false)
  })
})
