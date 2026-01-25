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
  Collapse,
  Tooltip,
  FormControlLabel,
  Switch,
} from '@mui/material'
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  ContentCopy as CopyIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
} from '@mui/icons-material'
import { apiKeysAPI, environmentsAPI, usersAPI } from '../services/api'
import { useAuthStore } from '../store/authStore'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { APIKey, APIKeyPermission, Environment, EnvironmentPermission } from '../types'

const createKeySchema = z.object({
  description: z.string().optional(),
  expiresIn: z.number().positive().optional(),
})

type CreateKeyFormData = z.infer<typeof createKeySchema>

export default function APIKeysPage() {
  const [open, setOpen] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)
  const [snackbarOpen, setSnackbarOpen] = useState(false)
  const [expandedRow, setExpandedRow] = useState<string | null>(null)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [keyToDelete, setKeyToDelete] = useState<APIKey | null>(null)
  const [scopePermissions, setScopePermissions] = useState(false)
  const [selectedPermissions, setSelectedPermissions] = useState<APIKeyPermission[]>([])
  const queryClient = useQueryClient()
  const { user: currentUser } = useAuthStore()

  const { data, isLoading } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiKeysAPI.list(),
  })

  const { data: environmentsData } = useQuery({
    queryKey: ['environments'],
    queryFn: () => environmentsAPI.list({ limit: 100 }),
    enabled: scopePermissions,
  })

  const { data: userPermissionsData } = useQuery({
    queryKey: ['user-permissions', currentUser?.id],
    queryFn: () => currentUser ? usersAPI.listPermissions(currentUser.id) : null,
    enabled: scopePermissions && !!currentUser && currentUser.role !== 'super_admin',
  })

  const createMutation = useMutation({
    mutationFn: ({ description, expiresIn, permissions }: { description?: string; expiresIn?: number; permissions?: APIKeyPermission[] }) =>
      apiKeysAPI.create({
        description,
        expires_in: expiresIn,
        permissions,
      }),
    onSuccess: (response) => {
      setNewKey(response.key)
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
      setOpen(false)
      setScopePermissions(false)
      setSelectedPermissions([])
      reset()
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) => apiKeysAPI.revoke(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
      setDeleteDialogOpen(false)
      setKeyToDelete(null)
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
      permissions: scopePermissions && selectedPermissions.length > 0 ? selectedPermissions : undefined,
    })
  }

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setSnackbarOpen(true)
  }

  const handleRevoke = (key: APIKey) => {
    setKeyToDelete(key)
    setDeleteDialogOpen(true)
  }

  const confirmRevoke = () => {
    if (keyToDelete) {
      revokeMutation.mutate(keyToDelete.id)
    }
  }

  const togglePermission = (envId: string) => {
    const existing = selectedPermissions.find((p) => p.environment_id === envId)
    if (existing) {
      setSelectedPermissions(selectedPermissions.filter((p) => p.environment_id !== envId))
    } else {
      setSelectedPermissions([...selectedPermissions, { environment_id: envId, permission: 'viewer' }])
    }
  }

  const updatePermissionLevel = (envId: string, level: 'viewer' | 'editor' | 'owner') => {
    setSelectedPermissions(
      selectedPermissions.map((p) =>
        p.environment_id === envId ? { ...p, permission: level } : p
      )
    )
  }

  const apiKeys = data?.api_keys as APIKey[] | undefined
  const environments = environmentsData?.environments as Environment[] | undefined
  const userPermissions = userPermissionsData?.permissions as EnvironmentPermission[] | undefined
  const isSuperAdmin = currentUser?.role === 'super_admin'

  // Get max permission level the user can grant for an environment
  const getMaxPermissionLevel = (envId: string): 'viewer' | 'editor' | 'owner' => {
    if (isSuperAdmin) return 'owner'
    const userPerm = userPermissions?.find((p) => p.environment_id === envId)
    return userPerm?.permission || 'viewer'
  }

  const permissionLevels = ['viewer', 'editor', 'owner'] as const
  const getAvailableLevels = (maxLevel: string) => {
    const maxIndex = permissionLevels.indexOf(maxLevel as typeof permissionLevels[number])
    return permissionLevels.slice(0, maxIndex + 1)
  }

  const getPermissionColor = (permission: string) => {
    switch (permission) {
      case 'owner': return 'success'
      case 'editor': return 'warning'
      case 'viewer': return 'info'
      default: return 'default'
    }
  }

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
              <TableCell width={40}></TableCell>
              <TableCell>Key Prefix</TableCell>
              <TableCell>Description</TableCell>
              <TableCell>Permissions</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Last Used</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={8} align="center">
                  Loading...
                </TableCell>
              </TableRow>
            ) : apiKeys?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} align="center">
                  No API keys found
                </TableCell>
              </TableRow>
            ) : (
              apiKeys?.map((key) => (
                <>
                  <TableRow key={key.id}>
                    <TableCell>
                      {key.permissions && key.permissions.length > 0 && (
                        <IconButton
                          size="small"
                          onClick={() => setExpandedRow(expandedRow === key.id ? null : key.id)}
                        >
                          {expandedRow === key.id ? <ExpandLessIcon /> : <ExpandMoreIcon />}
                        </IconButton>
                      )}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {key.key_prefix}...
                    </TableCell>
                    <TableCell>{key.description || '-'}</TableCell>
                    <TableCell>
                      {key.permissions && key.permissions.length > 0 ? (
                        <Chip
                          label={`${key.permissions.length} env(s)`}
                          size="small"
                          variant="outlined"
                        />
                      ) : (
                        <Chip label="Full Access" size="small" color="primary" />
                      )}
                    </TableCell>
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
                    <TableCell align="right">
                      {!key.revoked_at && (
                        <Tooltip title="Revoke API Key">
                          <IconButton
                            size="small"
                            onClick={() => handleRevoke(key)}
                            color="error"
                          >
                            <DeleteIcon />
                          </IconButton>
                        </Tooltip>
                      )}
                    </TableCell>
                  </TableRow>
                  {key.permissions && key.permissions.length > 0 && (
                    <TableRow>
                      <TableCell colSpan={8} sx={{ py: 0 }}>
                        <Collapse in={expandedRow === key.id} timeout="auto" unmountOnExit>
                          <Box sx={{ py: 2, px: 4, bgcolor: 'grey.50' }}>
                            <Typography variant="subtitle2" gutterBottom>
                              Environment Permissions
                            </Typography>
                            <Box display="flex" flexWrap="wrap" gap={1}>
                              {key.permissions.map((perm) => (
                                <Chip
                                  key={perm.environment_id}
                                  label={`${perm.environment_id}: ${perm.permission}`}
                                  size="small"
                                  color={getPermissionColor(perm.permission)}
                                />
                              ))}
                            </Box>
                          </Box>
                        </Collapse>
                      </TableCell>
                    </TableRow>
                  )}
                </>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Create API Key Dialog */}
      <Dialog open={open} onClose={() => { setOpen(false); setScopePermissions(false); setSelectedPermissions([]); reset() }} maxWidth="md" fullWidth>
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
              sx={{ mb: 2 }}
            />

            <FormControlLabel
              control={
                <Switch
                  checked={scopePermissions}
                  onChange={(e) => {
                    setScopePermissions(e.target.checked)
                    if (!e.target.checked) {
                      setSelectedPermissions([])
                    }
                  }}
                />
              }
              label="Scope to specific environments"
            />

            {scopePermissions && (
              <Box sx={{ mt: 2 }}>
                <Typography variant="body2" color="textSecondary" sx={{ mb: 1 }}>
                  Select environments and permission levels for this API key.
                  {!isSuperAdmin && ' You can only grant permissions up to your own level.'}
                </Typography>
                <TableContainer component={Paper} variant="outlined">
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Environment</TableCell>
                        <TableCell>Status</TableCell>
                        <TableCell>Permission Level</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {environments?.map((env) => {
                        const selected = selectedPermissions.find((p) => p.environment_id === env.id)
                        const maxLevel = getMaxPermissionLevel(env.id)
                        const hasAccess = isSuperAdmin || userPermissions?.some((p) => p.environment_id === env.id)

                        if (!hasAccess) return null

                        return (
                          <TableRow key={env.id}>
                            <TableCell>
                              <FormControlLabel
                                control={
                                  <Switch
                                    size="small"
                                    checked={!!selected}
                                    onChange={() => togglePermission(env.id)}
                                  />
                                }
                                label={env.name}
                              />
                            </TableCell>
                            <TableCell>
                              <Chip
                                label={env.status}
                                size="small"
                                color={env.status === 'running' ? 'success' : 'default'}
                              />
                            </TableCell>
                            <TableCell>
                              {selected ? (
                                <Box display="flex" gap={0.5}>
                                  {getAvailableLevels(maxLevel).map((level) => (
                                    <Chip
                                      key={level}
                                      label={level}
                                      size="small"
                                      variant={selected.permission === level ? 'filled' : 'outlined'}
                                      color={selected.permission === level ? getPermissionColor(level) : 'default'}
                                      onClick={() => updatePermissionLevel(env.id, level)}
                                      sx={{ cursor: 'pointer' }}
                                    />
                                  ))}
                                </Box>
                              ) : (
                                <Typography variant="body2" color="textSecondary">
                                  Not selected
                                </Typography>
                              )}
                            </TableCell>
                          </TableRow>
                        )
                      })}
                      {(!environments || environments.length === 0) && (
                        <TableRow>
                          <TableCell colSpan={3} align="center">
                            No environments available
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                </TableContainer>
                {selectedPermissions.length > 0 && (
                  <Typography variant="caption" color="textSecondary" sx={{ mt: 1, display: 'block' }}>
                    {selectedPermissions.length} environment(s) selected
                  </Typography>
                )}
              </Box>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => { setOpen(false); setScopePermissions(false); setSelectedPermissions([]); reset() }}>Cancel</Button>
            <Button type="submit" variant="contained" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogActions>
        </form>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onClose={() => { setDeleteDialogOpen(false); setKeyToDelete(null) }}>
        <DialogTitle>Revoke API Key</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to revoke this API key?
            {keyToDelete?.description && (
              <> (<strong>{keyToDelete.description}</strong>)</>
            )}
          </Typography>
          <Typography variant="body2" color="textSecondary" sx={{ mt: 1 }}>
            This action cannot be undone. Any applications using this key will lose access.
          </Typography>
          {revokeMutation.isError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {revokeMutation.error instanceof Error
                ? revokeMutation.error.message
                : 'Failed to revoke API key'}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setDeleteDialogOpen(false); setKeyToDelete(null) }}>Cancel</Button>
          <Button
            onClick={confirmRevoke}
            color="error"
            variant="contained"
            disabled={revokeMutation.isPending}
          >
            {revokeMutation.isPending ? 'Revoking...' : 'Revoke'}
          </Button>
        </DialogActions>
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
