import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Box,
  Paper,
  Typography,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  ListItemIcon,
  Chip,
  CircularProgress,
  Alert,
  Button,
  TextField,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  Divider,
  Tooltip,
} from '@mui/material'
import {
  PlayArrow as PlayIcon,
  Stop as StopIcon,
  CheckCircle as SuccessIcon,
  Error as ErrorIcon,
  HourglassEmpty as PendingIcon,
  Schedule as QueuedIcon,
  Refresh as RefreshIcon,
  Cancel as CancelIcon,
} from '@mui/icons-material'
import { executionsAPI } from '../../services/api'
import { Execution, ExecutionStatus } from '../../types'

interface ExecutionPanelProps {
  environmentId: string
}

const statusConfig: Record<ExecutionStatus, { icon: React.ReactNode; color: 'default' | 'primary' | 'secondary' | 'success' | 'error' | 'info' | 'warning' }> = {
  pending: { icon: <PendingIcon fontSize="small" />, color: 'default' },
  queued: { icon: <QueuedIcon fontSize="small" />, color: 'info' },
  running: { icon: <CircularProgress size={16} />, color: 'primary' },
  completed: { icon: <SuccessIcon fontSize="small" />, color: 'success' },
  failed: { icon: <ErrorIcon fontSize="small" />, color: 'error' },
  cancelled: { icon: <CancelIcon fontSize="small" />, color: 'warning' },
}

export default function ExecutionPanel({ environmentId }: ExecutionPanelProps) {
  const queryClient = useQueryClient()
  const [selectedExecution, setSelectedExecution] = useState<string | null>(null)
  const [newCommandDialogOpen, setNewCommandDialogOpen] = useState(false)
  const [command, setCommand] = useState('')
  const [timeout, setTimeout] = useState('300')

  // Fetch executions list
  const { data: executionsData, isLoading: listLoading, error: listError } = useQuery({
    queryKey: ['executions', environmentId],
    queryFn: () => executionsAPI.list(environmentId, { limit: 50 }),
    refetchInterval: 5000, // Poll every 5 seconds for updates
  })

  // Fetch selected execution details
  const { data: executionDetail, isLoading: detailLoading } = useQuery({
    queryKey: ['execution', selectedExecution],
    queryFn: () => executionsAPI.get(selectedExecution!),
    enabled: !!selectedExecution,
    refetchInterval: (query) => {
      // Only poll if execution is still running
      const status = query.state.data?.status
      if (status === 'pending' || status === 'queued' || status === 'running') {
        return 2000
      }
      return false
    },
  })

  // Submit new execution mutation
  const submitMutation = useMutation({
    mutationFn: (data: { command: string[]; timeout?: number }) =>
      executionsAPI.submit(environmentId, data),
    onSuccess: (newExecution) => {
      queryClient.invalidateQueries({ queryKey: ['executions', environmentId] })
      setSelectedExecution(newExecution.id)
      setNewCommandDialogOpen(false)
      setCommand('')
    },
  })

  // Cancel execution mutation
  const cancelMutation = useMutation({
    mutationFn: (execId: string) => executionsAPI.cancel(execId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['executions', environmentId] })
      if (selectedExecution) {
        queryClient.invalidateQueries({ queryKey: ['execution', selectedExecution] })
      }
    },
  })

  // Auto-select first execution when list loads
  useEffect(() => {
    if (executionsData?.executions && executionsData.executions.length > 0 && !selectedExecution) {
      setSelectedExecution(executionsData.executions[0].id)
    }
  }, [executionsData, selectedExecution])

  const handleSubmitCommand = () => {
    const cmdParts = command.trim().split(/\s+/)
    if (cmdParts.length > 0 && cmdParts[0]) {
      submitMutation.mutate({
        command: cmdParts,
        timeout: parseInt(timeout) || 300,
      })
    }
  }

  const formatDuration = (ms?: number) => {
    if (!ms) return '-'
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
  }

  const formatTime = (dateStr?: string) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleTimeString()
  }

  if (listLoading) {
    return (
      <Box display="flex" justifyContent="center" p={4}>
        <CircularProgress />
      </Box>
    )
  }

  if (listError) {
    return (
      <Alert severity="error">
        Failed to load executions: {listError instanceof Error ? listError.message : 'Unknown error'}
      </Alert>
    )
  }

  const executions = executionsData?.executions || []

  return (
    <Box sx={{ display: 'flex', height: '600px' }}>
      {/* Executions List (Left Panel) */}
      <Paper sx={{ width: 300, overflow: 'auto', borderRight: 1, borderColor: 'divider' }}>
        <Box sx={{ p: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Typography variant="subtitle1" fontWeight="bold">
            Executions
          </Typography>
          <Box>
            <Tooltip title="Refresh">
              <IconButton 
                size="small" 
                onClick={() => queryClient.invalidateQueries({ queryKey: ['executions', environmentId] })}
              >
                <RefreshIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Button
              size="small"
              variant="contained"
              startIcon={<PlayIcon />}
              onClick={() => setNewCommandDialogOpen(true)}
            >
              Run
            </Button>
          </Box>
        </Box>
        <Divider />
        {executions.length === 0 ? (
          <Box sx={{ p: 2, textAlign: 'center' }}>
            <Typography variant="body2" color="text.secondary">
              No executions yet
            </Typography>
            <Button
              variant="outlined"
              startIcon={<PlayIcon />}
              onClick={() => setNewCommandDialogOpen(true)}
              sx={{ mt: 2 }}
            >
              Run Command
            </Button>
          </Box>
        ) : (
          <List dense disablePadding>
            {executions.map((exec: Execution) => (
              <ListItem key={exec.id} disablePadding>
                <ListItemButton
                  selected={selectedExecution === exec.id}
                  onClick={() => setSelectedExecution(exec.id)}
                >
                  <ListItemIcon sx={{ minWidth: 36 }}>
                    {statusConfig[exec.status]?.icon}
                  </ListItemIcon>
                  <ListItemText
                    primary={exec.id}
                    secondary={formatTime(exec.created_at)}
                    primaryTypographyProps={{ variant: 'body2', fontFamily: 'monospace' }}
                    secondaryTypographyProps={{ variant: 'caption' }}
                  />
                  <Chip
                    label={exec.status}
                    size="small"
                    color={statusConfig[exec.status]?.color || 'default'}
                    sx={{ fontSize: '0.65rem', height: 20 }}
                  />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        )}
      </Paper>

      {/* Execution Detail (Right Panel) */}
      <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {selectedExecution && executionDetail ? (
          <>
            {/* Execution Header */}
            <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                <Typography variant="h6" fontFamily="monospace">
                  {executionDetail.id}
                </Typography>
                <Box>
                  <Chip
                    label={executionDetail.status}
                    color={statusConfig[executionDetail.status]?.color || 'default'}
                    icon={statusConfig[executionDetail.status]?.icon as React.ReactElement}
                    sx={{ mr: 1 }}
                  />
                  {(executionDetail.status === 'pending' || 
                    executionDetail.status === 'queued' || 
                    executionDetail.status === 'running') && (
                    <Button
                      size="small"
                      color="error"
                      variant="outlined"
                      startIcon={<StopIcon />}
                      onClick={() => cancelMutation.mutate(executionDetail.id)}
                      disabled={cancelMutation.isPending}
                    >
                      Cancel
                    </Button>
                  )}
                </Box>
              </Box>
              <Box sx={{ display: 'flex', gap: 3, color: 'text.secondary', fontSize: '0.875rem' }}>
                <span>Created: {formatTime(executionDetail.created_at)}</span>
                {executionDetail.started_at && <span>Started: {formatTime(executionDetail.started_at)}</span>}
                {executionDetail.completed_at && <span>Completed: {formatTime(executionDetail.completed_at)}</span>}
                {executionDetail.duration_ms && <span>Duration: {formatDuration(executionDetail.duration_ms)}</span>}
                {executionDetail.exit_code !== undefined && (
                  <span>
                    Exit Code:{' '}
                    <Chip 
                      label={executionDetail.exit_code} 
                      size="small" 
                      color={executionDetail.exit_code === 0 ? 'success' : 'error'}
                      sx={{ height: 18, fontSize: '0.75rem' }}
                    />
                  </span>
                )}
              </Box>
            </Box>

            {/* Execution Output */}
            <Box sx={{ flex: 1, overflow: 'auto', p: 0 }}>
              {detailLoading && !executionDetail ? (
                <Box display="flex" justifyContent="center" p={4}>
                  <CircularProgress />
                </Box>
              ) : executionDetail.error ? (
                <Alert severity="error" sx={{ m: 2 }}>
                  {executionDetail.error}
                </Alert>
              ) : executionDetail.status === 'pending' || executionDetail.status === 'queued' ? (
                <Box sx={{ p: 4, textAlign: 'center' }}>
                  <CircularProgress sx={{ mb: 2 }} />
                  <Typography color="text.secondary">
                    {executionDetail.status === 'pending' ? 'Waiting to start...' : 'Queued, waiting for available slot...'}
                  </Typography>
                </Box>
              ) : executionDetail.status === 'running' ? (
                <Box sx={{ p: 4, textAlign: 'center' }}>
                  <CircularProgress sx={{ mb: 2 }} />
                  <Typography color="text.secondary">
                    Executing command...
                  </Typography>
                </Box>
              ) : (
                <Box
                  sx={{
                    fontFamily: 'monospace',
                    fontSize: '0.875rem',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                    p: 2,
                    backgroundColor: '#1e1e1e',
                    color: '#d4d4d4',
                    minHeight: '100%',
                  }}
                >
                  {executionDetail.stdout || (executionDetail.status === 'completed' ? '(no output)' : '')}
                  {executionDetail.stderr && (
                    <Box sx={{ color: '#f48771', mt: 1 }}>
                      {executionDetail.stderr}
                    </Box>
                  )}
                </Box>
              )}
            </Box>
          </>
        ) : (
          <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Typography color="text.secondary">
              Select an execution to view details
            </Typography>
          </Box>
        )}
      </Box>

      {/* New Command Dialog */}
      <Dialog open={newCommandDialogOpen} onClose={() => setNewCommandDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Run New Command</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            The command will run in a new isolated pod using this environment's configuration.
          </Typography>
          <TextField
            autoFocus
            fullWidth
            label="Command"
            placeholder="e.g., python -c 'print(hello)'"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            sx={{ mb: 2 }}
            helperText="Enter the full command with arguments"
          />
          <TextField
            fullWidth
            label="Timeout (seconds)"
            type="number"
            value={timeout}
            onChange={(e) => setTimeout(e.target.value)}
            helperText="Maximum execution time (default: 300, max: 3600)"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setNewCommandDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={handleSubmitCommand}
            disabled={!command.trim() || submitMutation.isPending}
            startIcon={submitMutation.isPending ? <CircularProgress size={16} /> : <PlayIcon />}
          >
            {submitMutation.isPending ? 'Submitting...' : 'Run'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
