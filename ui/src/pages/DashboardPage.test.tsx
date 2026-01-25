import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { render } from '../test/test-utils'
import DashboardPage from './DashboardPage'

// Mock the API with a default implementation
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
        {
          id: 'env-789',
          name: 'test-env-3',
          status: 'failed',
          image: 'golang:1.22',
          resources: { cpu: '2', memory: '2Gi', storage: '5Gi' },
          created_at: '2026-01-22T12:00:00Z',
        },
      ],
      total: 3,
      limit: 100,
      offset: 0,
    }),
  },
  metricsAPI: {
    getGlobal: vi.fn().mockResolvedValue({ metrics: [] }),
    getEnvironment: vi.fn().mockResolvedValue({ metrics: [] }),
  },
}))

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the dashboard title', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeInTheDocument()
    })
  })

  it('displays total environments label', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText('Total Environments')).toBeInTheDocument()
    })
  })

  it('displays running environments label', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText('Running')).toBeInTheDocument()
    })
  })

  it('displays pending environments label', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText('Pending')).toBeInTheDocument()
    })
  })

  it('displays failed environments label', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText('Failed')).toBeInTheDocument()
    })
  })

  it('renders CPU usage card', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText('Current CPU Usage')).toBeInTheDocument()
    })
  })

  it('renders Memory usage card', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText('Current Memory Usage')).toBeInTheDocument()
    })
  })

  it('renders CPU chart section', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText(/CPU Usage Over Time/)).toBeInTheDocument()
    })
  })

  it('renders Memory chart section', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText(/Memory Usage Over Time/)).toBeInTheDocument()
    })
  })

  it('renders Running Sandboxes chart section', async () => {
    render(<DashboardPage />)
    
    await waitFor(() => {
      expect(screen.getByText('Running Sandboxes Over Time')).toBeInTheDocument()
    })
  })
})
