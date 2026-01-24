import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { render } from '../test/test-utils'
import LoginPage from './LoginPage'

// Mock useNavigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders login form', () => {
    render(<LoginPage />)

    expect(screen.getByText('AgentBox')).toBeInTheDocument()
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('has tabs for local and google login', () => {
    render(<LoginPage />)

    expect(screen.getByRole('tab', { name: /local login/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /google login/i })).toBeInTheDocument()
  })

  it('allows typing in username and password fields', async () => {
    const user = userEvent.setup()
    render(<LoginPage />)

    const usernameInput = screen.getByLabelText(/username/i)
    const passwordInput = screen.getByLabelText(/password/i)

    await user.type(usernameInput, 'testuser')
    await user.type(passwordInput, 'testpass')

    expect(usernameInput).toHaveValue('testuser')
    expect(passwordInput).toHaveValue('testpass')
  })

  it('displays the sandbox management subtitle', () => {
    render(<LoginPage />)
    
    expect(screen.getByText('Sandbox Management Platform')).toBeInTheDocument()
  })
})
