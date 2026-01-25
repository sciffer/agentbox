import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
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
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Typography,
  Chip,
  Snackbar,
} from '@mui/material'
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  ContentCopy as CopyIcon,
} from '@mui/icons-material'
import { apiKeysAPI } from '../services/api'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { APIKey, CreateAPIKeyData } from '../types'

const createKeySchema = z.object({
  description: z.string().optional(),
  expiresIn: z.number().positive().optional(),
})

type CreateKeyFormData = z.infer<typeof createKeySchema>

export default function APIKeysPage() {
  const [open, setOpen] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)
  const [snackbarOpen, setSnackbarOpen] = useState(false)
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiKeysAPI.list(),
  })

  const createMutation = useMutation({
    mutationFn: (formData: CreateAPIKeyData) => apiKeysAPI.create(formData.description, formData.expiresIn),
    onSuccess: (response) => {
      setNewKey(response.key)
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
      setOpen(false)
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) => apiKeysAPI.revoke(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
    },
  })

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<CreateKeyFormData>({
    resolver: zodResolver(createKeySchema),
  })

  const onSubmit = (formData: CreateKeyFormData) => {
    createMutation.mutate({
      description: formData.description || undefined,
      expiresIn: formData.expiresIn ? formData.expiresIn : undefined,
    })
  }

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setSnackbarOpen(true)
  }

  const handleRevoke = (id: string) => {
    if (window.confirm('Are you sure you want to revoke this API key?')) {
      revokeMutation.mutate(id)
    }
  }

  const apiKeys = data?.api_keys as APIKey[] | undefined

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="h4">API Keys</Typography>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => setOpen(true)}
        >
          Create API Key
        </Button>
      </Box>

      {newKey && (
        <Alert
          severity="success"
          onClose={() => setNewKey(null)}
          sx={{ mb: 2 }}
        >
          <Typography variant="body2" gutterBottom>
            <strong>API Key created! Copy it now - you won't be able to see it again.</strong>
          </Typography>
          <Box display="flex" alignItems="center" gap={1} mt={1}>
            <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
              {newKey}
            </Typography>
            <IconButton size="small" onClick={() => handleCopy(newKey)}>
              <CopyIcon fontSize="small" />
            </IconButton>
          </Box>
        </Alert>
      )}

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Key Prefix</TableCell>
              <TableCell>Description</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Last Used</TableCell>
              <TableCell>Status</TableCell>
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
            ) : apiKeys?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  No API keys found
                </TableCell>
              </TableRow>
            ) : (
              apiKeys?.map((key) => (
                <TableRow key={key.id}>
                  <TableCell sx={{ fontFamily: 'monospace' }}>
                    {key.key_prefix}...
                  </TableCell>
                  <TableCell>{key.description || '-'}</TableCell>
                  <TableCell>
                    {new Date(key.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    {key.last_used
                      ? new Date(key.last_used).toLocaleString()
                      : 'Never'}
                  </TableCell>
                  <TableCell>
                    {key.revoked_at ? (
                      <Chip label="Revoked" color="error" size="small" />
                    ) : key.expires_at && new Date(key.expires_at) < new Date() ? (
                      <Chip label="Expired" color="warning" size="small" />
                    ) : (
                      <Chip label="Active" color="success" size="small" />
                    )}
                  </TableCell>
                  <TableCell>
                    {!key.revoked_at && (
                      <IconButton
                        size="small"
                        onClick={() => handleRevoke(key.id)}
                        color="error"
                      >
                        <DeleteIcon />
                      </IconButton>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="sm" fullWidth>
        <form onSubmit={handleSubmit(onSubmit)}>
          <DialogTitle>Create API Key</DialogTitle>
          <DialogContent>
            {createMutation.isError && (
              <Alert severity="error" sx={{ mb: 2 }}>
                {createMutation.error instanceof Error
                  ? createMutation.error.message
                  : 'Failed to create API key'}
              </Alert>
            )}
            <TextField
              margin="dense"
              label="Description"
              fullWidth
              {...register('description')}
              error={!!errors.description}
              helperText={errors.description?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="Expires In (days)"
              fullWidth
              type="number"
              {...register('expiresIn', { valueAsNumber: true })}
              error={!!errors.expiresIn}
              helperText={errors.expiresIn?.message || 'Leave empty for no expiration'}
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

      <Snackbar
        open={snackbarOpen}
        autoHideDuration={2000}
        onClose={() => setSnackbarOpen(false)}
        message="Copied to clipboard"
      />
    </Box>
  )
}
