import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
  Box,
  Button,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Typography,
} from '@mui/material'
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Visibility as ViewIcon,
  Terminal as TerminalIcon,
} from '@mui/icons-material'
import { environmentsAPI } from '../services/api'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const createEnvSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  image: z.string().min(1, 'Image is required'),
  cpu: z.string().min(1, 'CPU is required'),
  memory: z.string().min(1, 'Memory is required'),
  storage: z.string().min(1, 'Storage is required'),
})

type CreateEnvFormData = z.infer<typeof createEnvSchema>

function StatusChip({ status }: { status: string }) {
  const colors: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
    running: 'success',
    pending: 'warning',
    failed: 'error',
    terminated: 'default',
  }
  return <Chip label={status} color={colors[status] || 'default'} size="small" />
}

export default function EnvironmentsPage() {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['environments'],
    queryFn: () => environmentsAPI.list(),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => environmentsAPI.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['environments'] })
    },
  })

  const createMutation = useMutation({
    mutationFn: (data: any) => environmentsAPI.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['environments'] })
      setOpen(false)
    },
  })

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<CreateEnvFormData>({
    resolver: zodResolver(createEnvSchema),
    defaultValues: {
      cpu: '500m',
      memory: '512Mi',
      storage: '1Gi',
    },
  })

  const onSubmit = (data: CreateEnvFormData) => {
    createMutation.mutate({
      name: data.name,
      image: data.image,
      resources: {
        cpu: data.cpu,
        memory: data.memory,
        storage: data.storage,
      },
    })
  }

  const handleDelete = (id: string) => {
    if (window.confirm('Are you sure you want to delete this environment?')) {
      deleteMutation.mutate(id)
    }
  }

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="h4">Environments</Typography>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => setOpen(true)}
        >
          Create Environment
        </Button>
      </Box>

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Image</TableCell>
              <TableCell>Resources</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  Loading...
                </TableCell>
              </TableRow>
            ) : data?.environments?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  No environments found
                </TableCell>
              </TableRow>
            ) : (
              data?.environments?.map((env: any) => (
                <TableRow key={env.id}>
                  <TableCell>{env.name}</TableCell>
                  <TableCell>
                    <StatusChip status={env.status} />
                  </TableCell>
                  <TableCell>{env.image}</TableCell>
                  <TableCell>
                    {env.resources?.cpu} / {env.resources?.memory}
                  </TableCell>
                  <TableCell>
                    {new Date(env.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <IconButton
                      size="small"
                      onClick={() => navigate(`/environments/${env.id}`)}
                    >
                      <ViewIcon />
                    </IconButton>
                    <IconButton
                      size="small"
                      onClick={() => handleDelete(env.id)}
                      color="error"
                    >
                      <DeleteIcon />
                    </IconButton>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="sm" fullWidth>
        <form onSubmit={handleSubmit(onSubmit)}>
          <DialogTitle>Create Environment</DialogTitle>
          <DialogContent>
            {createMutation.isError && (
              <Alert severity="error" sx={{ mb: 2 }}>
                {createMutation.error instanceof Error
                  ? createMutation.error.message
                  : 'Failed to create environment'}
              </Alert>
            )}
            <TextField
              margin="dense"
              label="Name"
              fullWidth
              required
              {...register('name')}
              error={!!errors.name}
              helperText={errors.name?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="Image"
              fullWidth
              required
              placeholder="python:3.11-slim"
              {...register('image')}
              error={!!errors.image}
              helperText={errors.image?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="CPU"
              fullWidth
              required
              placeholder="500m"
              {...register('cpu')}
              error={!!errors.cpu}
              helperText={errors.cpu?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="Memory"
              fullWidth
              required
              placeholder="512Mi"
              {...register('memory')}
              error={!!errors.memory}
              helperText={errors.memory?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="Storage"
              fullWidth
              required
              placeholder="1Gi"
              {...register('storage')}
              error={!!errors.storage}
              helperText={errors.storage?.message}
            />
          </DialogContent>
          <DialogActions>
            <Button onClick={() => { setOpen(false); reset() }}>Cancel</Button>
            <Button type="submit" variant="contained" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </Box>
  )
}
