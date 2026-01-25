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
  Chip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Typography,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  IconButton,
  Tooltip,
  Menu,
  ListItemIcon,
  ListItemText,
  Divider,
} from '@mui/material'
import {
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  MoreVert as MoreVertIcon,
  Security as SecurityIcon,
} from '@mui/icons-material'
import { usersAPI, environmentsAPI } from '../services/api'
import { useAuthStore } from '../store/authStore'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { User, CreateUserData, UpdateUserData, EnvironmentPermission, Environment } from '../types'

const createUserSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  email: z.string().email('Invalid email').optional().or(z.literal('')),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  role: z.enum(['user', 'admin', 'super_admin']),
  status: z.enum(['active', 'inactive']),
})

const updateUserSchema = z.object({
  username: z.string().min(1, 'Username is required').optional(),
  email: z.string().email('Invalid email').optional().or(z.literal('')),
  password: z.string().min(8, 'Password must be at least 8 characters').optional().or(z.literal('')),
  role: z.enum(['user', 'admin', 'super_admin']).optional(),
  status: z.enum(['active', 'inactive']).optional(),
})

type CreateUserFormData = z.infer<typeof createUserSchema>
type UpdateUserFormData = z.infer<typeof updateUserSchema>

export default function UsersPage() {
  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [permissionsOpen, setPermissionsOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [menuAnchor, setMenuAnchor] = useState<null | HTMLElement>(null)
  const [menuUser, setMenuUser] = useState<User | null>(null)
  const { user: currentUser } = useAuthStore()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => usersAPI.list(),
  })

  const { data: environmentsData } = useQuery({
    queryKey: ['environments'],
    queryFn: () => environmentsAPI.list({ limit: 100 }),
  })

  const { data: permissionsData, refetch: refetchPermissions } = useQuery({
    queryKey: ['user-permissions', selectedUser?.id],
    queryFn: () => selectedUser ? usersAPI.listPermissions(selectedUser.id) : null,
    enabled: !!selectedUser && permissionsOpen,
  })

  const createMutation = useMutation({
    mutationFn: (formData: CreateUserData) => usersAPI.create(formData),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setCreateOpen(false)
      createReset()
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateUserData }) => usersAPI.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setEditOpen(false)
      setSelectedUser(null)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => usersAPI.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setDeleteOpen(false)
      setSelectedUser(null)
    },
  })

  const grantPermissionMutation = useMutation({
    mutationFn: ({ userId, envId, permission }: { userId: string; envId: string; permission: string }) =>
      usersAPI.grantPermission(userId, { environment_id: envId, permission: permission as 'viewer' | 'editor' | 'owner' }),
    onSuccess: () => {
      refetchPermissions()
    },
  })

  const updatePermissionMutation = useMutation({
    mutationFn: ({ userId, envId, permission }: { userId: string; envId: string; permission: string }) =>
      usersAPI.updatePermission(userId, envId, permission),
    onSuccess: () => {
      refetchPermissions()
    },
  })

  const revokePermissionMutation = useMutation({
    mutationFn: ({ userId, envId }: { userId: string; envId: string }) =>
      usersAPI.revokePermission(userId, envId),
    onSuccess: () => {
      refetchPermissions()
    },
  })

  const {
    register: createRegister,
    handleSubmit: createHandleSubmit,
    formState: { errors: createErrors },
    reset: createReset,
  } = useForm<CreateUserFormData>({
    resolver: zodResolver(createUserSchema),
    defaultValues: {
      role: 'user',
      status: 'active',
    },
  })

  const {
    register: editRegister,
    handleSubmit: editHandleSubmit,
    formState: { errors: editErrors },
    reset: editReset,
    setValue: editSetValue,
  } = useForm<UpdateUserFormData>({
    resolver: zodResolver(updateUserSchema),
  })

  const onCreateSubmit = (formData: CreateUserFormData) => {
    createMutation.mutate({
      username: formData.username,
      email: formData.email || undefined,
      password: formData.password,
      role: formData.role,
      status: formData.status,
    })
  }

  const onEditSubmit = (formData: UpdateUserFormData) => {
    if (!selectedUser) return
    const updateData: UpdateUserData = {}
    if (formData.username && formData.username !== selectedUser.username) {
      updateData.username = formData.username
    }
    if (formData.email !== undefined) {
      updateData.email = formData.email || undefined
    }
    if (formData.password) {
      updateData.password = formData.password
    }
    if (formData.role && formData.role !== selectedUser.role) {
      updateData.role = formData.role
    }
    if (formData.status && formData.status !== selectedUser.status) {
      updateData.status = formData.status
    }
    updateMutation.mutate({ id: selectedUser.id, data: updateData })
  }

  const handleMenuOpen = (event: React.MouseEvent<HTMLElement>, user: User) => {
    setMenuAnchor(event.currentTarget)
    setMenuUser(user)
  }

  const handleMenuClose = () => {
    setMenuAnchor(null)
    setMenuUser(null)
  }

  const handleEdit = (user: User) => {
    setSelectedUser(user)
    editSetValue('username', user.username)
    editSetValue('email', user.email || '')
    editSetValue('role', user.role as 'user' | 'admin' | 'super_admin')
    editSetValue('status', user.status as 'active' | 'inactive')
    editSetValue('password', '')
    setEditOpen(true)
    handleMenuClose()
  }

  const handleDelete = (user: User) => {
    setSelectedUser(user)
    setDeleteOpen(true)
    handleMenuClose()
  }

  const handlePermissions = (user: User) => {
    setSelectedUser(user)
    setPermissionsOpen(true)
    handleMenuClose()
  }

  const confirmDelete = () => {
    if (selectedUser) {
      deleteMutation.mutate(selectedUser.id)
    }
  }

  const canManageUsers = currentUser?.role === 'super_admin' || currentUser?.role === 'admin'
  const isSuperAdmin = currentUser?.role === 'super_admin'

  if (!canManageUsers) {
    return (
      <Alert severity="error">You don't have permission to view this page.</Alert>
    )
  }

  const users = data?.users as User[] | undefined
  const environments = environmentsData?.environments as Environment[] | undefined
  const permissions = permissionsData?.permissions as EnvironmentPermission[] | undefined

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="h4">Users</Typography>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => setCreateOpen(true)}
        >
          Create User
        </Button>
      </Box>

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Username</TableCell>
              <TableCell>Email</TableCell>
              <TableCell>Role</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  Loading...
                </TableCell>
              </TableRow>
            ) : users?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  No users found
                </TableCell>
              </TableRow>
            ) : (
              users?.map((user) => (
                <TableRow key={user.id}>
                  <TableCell>{user.username}</TableCell>
                  <TableCell>{user.email || '-'}</TableCell>
                  <TableCell>
                    <Chip 
                      label={user.role} 
                      size="small"
                      color={user.role === 'super_admin' ? 'error' : user.role === 'admin' ? 'primary' : 'default'}
                    />
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={user.status}
                      color={user.status === 'active' ? 'success' : 'default'}
                      size="small"
                    />
                  </TableCell>
                  <TableCell>
                    {new Date(user.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell align="right">
                    <Tooltip title="Actions">
                      <IconButton
                        size="small"
                        onClick={(e) => handleMenuOpen(e, user)}
                      >
                        <MoreVertIcon />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Action Menu */}
      <Menu
        anchorEl={menuAnchor}
        open={Boolean(menuAnchor)}
        onClose={handleMenuClose}
      >
        <MenuItem onClick={() => menuUser && handleEdit(menuUser)}>
          <ListItemIcon>
            <EditIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Edit</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => menuUser && handlePermissions(menuUser)}>
          <ListItemIcon>
            <SecurityIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Permissions</ListItemText>
        </MenuItem>
        <Divider />
        <MenuItem
          onClick={() => menuUser && handleDelete(menuUser)}
          disabled={menuUser?.id === currentUser?.id || (!isSuperAdmin && menuUser?.role === 'super_admin')}
        >
          <ListItemIcon>
            <DeleteIcon fontSize="small" color="error" />
          </ListItemIcon>
          <ListItemText sx={{ color: 'error.main' }}>Delete</ListItemText>
        </MenuItem>
      </Menu>

      {/* Create User Dialog */}
      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} maxWidth="sm" fullWidth>
        <form onSubmit={createHandleSubmit(onCreateSubmit)}>
          <DialogTitle>Create User</DialogTitle>
          <DialogContent>
            {createMutation.isError && (
              <Alert severity="error" sx={{ mb: 2 }}>
                {createMutation.error instanceof Error
                  ? createMutation.error.message
                  : 'Failed to create user'}
              </Alert>
            )}
            <TextField
              margin="dense"
              label="Username"
              fullWidth
              required
              {...createRegister('username')}
              error={!!createErrors.username}
              helperText={createErrors.username?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="Email"
              fullWidth
              type="email"
              {...createRegister('email')}
              error={!!createErrors.email}
              helperText={createErrors.email?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="Password"
              fullWidth
              required
              type="password"
              {...createRegister('password')}
              error={!!createErrors.password}
              helperText={createErrors.password?.message}
              sx={{ mb: 2 }}
            />
            <FormControl fullWidth margin="dense" sx={{ mb: 2 }}>
              <InputLabel>Role</InputLabel>
              <Select
                label="Role"
                {...createRegister('role')}
                defaultValue="user"
              >
                <MenuItem value="user">User</MenuItem>
                <MenuItem value="admin">Admin</MenuItem>
                {isSuperAdmin && <MenuItem value="super_admin">Super Admin</MenuItem>}
              </Select>
            </FormControl>
            <FormControl fullWidth margin="dense">
              <InputLabel>Status</InputLabel>
              <Select
                label="Status"
                {...createRegister('status')}
                defaultValue="active"
              >
                <MenuItem value="active">Active</MenuItem>
                <MenuItem value="inactive">Inactive</MenuItem>
              </Select>
            </FormControl>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => { setCreateOpen(false); createReset() }}>Cancel</Button>
            <Button type="submit" variant="contained" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogActions>
        </form>
      </Dialog>

      {/* Edit User Dialog */}
      <Dialog open={editOpen} onClose={() => { setEditOpen(false); setSelectedUser(null) }} maxWidth="sm" fullWidth>
        <form onSubmit={editHandleSubmit(onEditSubmit)}>
          <DialogTitle>Edit User: {selectedUser?.username}</DialogTitle>
          <DialogContent>
            {updateMutation.isError && (
              <Alert severity="error" sx={{ mb: 2 }}>
                {updateMutation.error instanceof Error
                  ? updateMutation.error.message
                  : 'Failed to update user'}
              </Alert>
            )}
            <TextField
              margin="dense"
              label="Username"
              fullWidth
              {...editRegister('username')}
              error={!!editErrors.username}
              helperText={editErrors.username?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="Email"
              fullWidth
              type="email"
              {...editRegister('email')}
              error={!!editErrors.email}
              helperText={editErrors.email?.message}
              sx={{ mb: 2 }}
            />
            <TextField
              margin="dense"
              label="New Password (leave blank to keep current)"
              fullWidth
              type="password"
              {...editRegister('password')}
              error={!!editErrors.password}
              helperText={editErrors.password?.message}
              sx={{ mb: 2 }}
            />
            <FormControl fullWidth margin="dense" sx={{ mb: 2 }}>
              <InputLabel>Role</InputLabel>
              <Select
                label="Role"
                {...editRegister('role')}
                defaultValue={selectedUser?.role || 'user'}
                disabled={!isSuperAdmin && selectedUser?.role === 'super_admin'}
              >
                <MenuItem value="user">User</MenuItem>
                <MenuItem value="admin">Admin</MenuItem>
                {isSuperAdmin && <MenuItem value="super_admin">Super Admin</MenuItem>}
              </Select>
            </FormControl>
            <FormControl fullWidth margin="dense">
              <InputLabel>Status</InputLabel>
              <Select
                label="Status"
                {...editRegister('status')}
                defaultValue={selectedUser?.status || 'active'}
              >
                <MenuItem value="active">Active</MenuItem>
                <MenuItem value="inactive">Inactive</MenuItem>
              </Select>
            </FormControl>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => { setEditOpen(false); setSelectedUser(null); editReset() }}>Cancel</Button>
            <Button type="submit" variant="contained" disabled={updateMutation.isPending}>
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
          </DialogActions>
        </form>
      </Dialog>

      {/* Delete User Dialog */}
      <Dialog open={deleteOpen} onClose={() => { setDeleteOpen(false); setSelectedUser(null) }}>
        <DialogTitle>Delete User</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete user <strong>{selectedUser?.username}</strong>?
            This action cannot be undone and will also delete all their API keys and permissions.
          </Typography>
          {deleteMutation.isError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {deleteMutation.error instanceof Error
                ? deleteMutation.error.message
                : 'Failed to delete user'}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setDeleteOpen(false); setSelectedUser(null) }}>Cancel</Button>
          <Button
            onClick={confirmDelete}
            color="error"
            variant="contained"
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Permissions Dialog */}
      <Dialog
        open={permissionsOpen}
        onClose={() => { setPermissionsOpen(false); setSelectedUser(null) }}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>
          Environment Permissions: {selectedUser?.username}
          {selectedUser?.role === 'super_admin' && (
            <Chip label="Super Admin - Full Access" color="error" size="small" sx={{ ml: 2 }} />
          )}
        </DialogTitle>
        <DialogContent>
          {selectedUser?.role === 'super_admin' ? (
            <Alert severity="info" sx={{ mb: 2 }}>
              Super admins have implicit access to all environments. No explicit permissions needed.
            </Alert>
          ) : (
            <>
              <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
                Grant permissions to specific environments. Users without explicit permissions cannot access environments.
              </Typography>
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Environment</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Permission</TableCell>
                      <TableCell align="right">Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {environments?.map((env) => {
                      const perm = permissions?.find((p) => p.environment_id === env.id)
                      return (
                        <TableRow key={env.id}>
                          <TableCell>{env.name}</TableCell>
                          <TableCell>
                            <Chip
                              label={env.status}
                              size="small"
                              color={env.status === 'running' ? 'success' : 'default'}
                            />
                          </TableCell>
                          <TableCell>
                            {perm ? (
                              <Select
                                size="small"
                                value={perm.permission}
                                onChange={(e) => {
                                  if (selectedUser) {
                                    updatePermissionMutation.mutate({
                                      userId: selectedUser.id,
                                      envId: env.id,
                                      permission: e.target.value,
                                    })
                                  }
                                }}
                              >
                                <MenuItem value="viewer">Viewer</MenuItem>
                                <MenuItem value="editor">Editor</MenuItem>
                                <MenuItem value="owner">Owner</MenuItem>
                              </Select>
                            ) : (
                              <Chip label="No Access" size="small" variant="outlined" />
                            )}
                          </TableCell>
                          <TableCell align="right">
                            {perm ? (
                              <Button
                                size="small"
                                color="error"
                                onClick={() => {
                                  if (selectedUser) {
                                    revokePermissionMutation.mutate({
                                      userId: selectedUser.id,
                                      envId: env.id,
                                    })
                                  }
                                }}
                              >
                                Revoke
                              </Button>
                            ) : (
                              <Button
                                size="small"
                                variant="outlined"
                                onClick={() => {
                                  if (selectedUser) {
                                    grantPermissionMutation.mutate({
                                      userId: selectedUser.id,
                                      envId: env.id,
                                      permission: 'viewer',
                                    })
                                  }
                                }}
                              >
                                Grant
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      )
                    })}
                    {(!environments || environments.length === 0) && (
                      <TableRow>
                        <TableCell colSpan={4} align="center">
                          No environments available
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setPermissionsOpen(false); setSelectedUser(null) }}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
