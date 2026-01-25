import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { render } from '../test/test-utils'
import EnvironmentsPage from './EnvironmentsPage'

// Mock the API
vi.mock('../services/api', () => ({
  environmentsAPI: {
    list: vi.fn().mockResolvedValue({
      environments: [
        {
          id: 'env-123',
          name: 'test-env',
          status: 'running',
          image: 'python:3.11-slim',
          resources: { cpu: '500m', memory: '512Mi', storage: '1Gi' },
          created_at: '2026-01-22T10:00:00Z',
        },
        {
          id: 'env-456',
          name: 'test-env-2',
          status: 'pending',
          image: 'node:18',
          resources: { cpu: '1', memory: '1Gi', storage: '2Gi' },
          created_at: '2026-01-22T11:00:00Z',
        },
      ],
      total: 2,
      limit: 100,
      offset: 0,
    }),
    create: vi.fn().mockResolvedValue({ id: 'env-new', name: 'new-env' }),
    delete: vi.fn().mockResolvedValue(undefined),
  },
}))

describe('EnvironmentsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the environments page title', async () => {
    render(<EnvironmentsPage />)
    expect(screen.getByRole('heading', { name: 'Environments' })).toBeInTheDocument()
  })

  it('renders the create environment button', () => {
    render(<EnvironmentsPage />)
    expect(screen.getByRole('button', { name: /create environment/i })).toBeInTheDocument()
  })

  it('displays environments in table', async () => {
    render(<EnvironmentsPage />)
    
    await waitFor(() => {
      expect(screen.getByText('test-env')).toBeInTheDocument()
    })
    expect(screen.getByText('test-env-2')).toBeInTheDocument()
  })

  it('opens create dialog when button is clicked', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    const createButton = screen.getByRole('button', { name: /create environment/i })
    await user.click(createButton)
    
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('shows basic settings fields in create dialog', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    // Check for form fields within the dialog
    const dialog = screen.getByRole('dialog')
    expect(dialog).toBeInTheDocument()
    
    // Use more specific queries
    expect(screen.getByRole('textbox', { name: /name/i })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: /image/i })).toBeInTheDocument()
  })

  it('shows node scheduling accordion in create dialog', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    expect(screen.getByText('Node Scheduling')).toBeInTheDocument()
  })

  it('shows add toleration button after expanding node scheduling', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    // Find and click the Node Scheduling accordion
    const nodeSchedulingHeader = screen.getByText('Node Scheduling')
    await user.click(nodeSchedulingHeader)
    
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add toleration/i })).toBeInTheDocument()
    })
  })

  it('can add multiple tolerations', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    // Wait for dialog to be visible
    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })
    
    // Expand Node Scheduling accordion and wait for content to be visible
    const nodeSchedulingHeader = screen.getByText('Node Scheduling')
    await user.click(nodeSchedulingHeader)
    
    // Wait for accordion to expand and button to be visible
    const addButton = await screen.findByRole('button', { name: /add toleration/i })
    
    // Add first toleration
    await user.click(addButton)
    
    // Should have one toleration row (Key field)
    await waitFor(() => {
      expect(screen.getAllByRole('textbox', { name: /key/i }).length).toBe(1)
    }, { timeout: 3000 })
    
    // Add second toleration - re-query the button to ensure it's still in the DOM
    const addButton2 = screen.getByRole('button', { name: /add toleration/i })
    await user.click(addButton2)
    
    // Should have two toleration rows
    await waitFor(() => {
      expect(screen.getAllByRole('textbox', { name: /key/i }).length).toBe(2)
    }, { timeout: 3000 })
  }, 15000) // Increase test timeout to 15 seconds

  it('can fill in toleration fields', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    // Wait for dialog to be visible
    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })
    
    // Expand Node Scheduling
    const nodeSchedulingHeader = screen.getByText('Node Scheduling')
    await user.click(nodeSchedulingHeader)
    
    // Add a toleration
    const addButton = await screen.findByRole('button', { name: /add toleration/i })
    await user.click(addButton)
    
    // Wait for key input to appear and fill it
    const keyInput = await screen.findByRole('textbox', { name: /key/i })
    await user.type(keyInput, 'dedicated')
    
    await waitFor(() => {
      expect(keyInput).toHaveValue('dedicated')
    })
  }, 10000)

  it('shows runtime isolation accordion in create dialog', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    expect(screen.getByText('Runtime Isolation')).toBeInTheDocument()
  })

  it('shows network policy accordion in create dialog', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    expect(screen.getByText('Network Policy')).toBeInTheDocument()
  })

  it('shows security context accordion in create dialog', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    
    expect(screen.getByText('Security Context')).toBeInTheDocument()
  })

  it('closes dialog on cancel', async () => {
    const user = userEvent.setup()
    render(<EnvironmentsPage />)
    
    await user.click(screen.getByRole('button', { name: /create environment/i }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    
    await user.click(screen.getByRole('button', { name: /cancel/i }))
    
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })
})
