import { describe, it, expect } from 'vitest'
import { screen } from '@testing-library/react'
import { render } from '../test/test-utils'
import LoginPage from './LoginPage'

describe('LoginPage', () => {
  it('renders login form with title', () => {
    render(<LoginPage />)
    expect(screen.getByText('AgentBox')).toBeInTheDocument()
  })

  it('renders username input', () => {
    render(<LoginPage />)
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
  })

  it('renders password input', () => {
    render(<LoginPage />)
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
  })

  it('renders sign in button', () => {
    render(<LoginPage />)
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('displays the sandbox management subtitle', () => {
    render(<LoginPage />)
    expect(screen.getByText('Sandbox Management Platform')).toBeInTheDocument()
  })
})
