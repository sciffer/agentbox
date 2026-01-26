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
  Accordion,
  AccordionSummary,
  AccordionDetails,
  FormControlLabel,
  Switch,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Grid,
  Tooltip,
  Divider,
} from '@mui/material'
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Visibility as ViewIcon,
  ExpandMore as ExpandMoreIcon,
  Info as InfoIcon,
} from '@mui/icons-material'
import { environmentsAPI } from '../services/api'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Environment, CreateEnvironmentData, Toleration, IsolationConfig, PoolConfig } from '../types'

// Interface for toleration form entry
interface TolerationEntry {
  key: string
  operator: 'Exists' | 'Equal'
  value: string
  effect: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute' | ''
}

const createEnvSchema = z.object({
  name: z.string().min(1, 'Name is required').regex(/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, 'Must be lowercase alphanumeric with hyphens'),
  image: z.string().min(1, 'Image is required'),
  cpu: z.string().min(1, 'CPU is required'),
  memory: z.string().min(1, 'Memory is required'),
  storage: z.string().min(1, 'Storage is required'),
  timeout: z.number().optional(),
  // Node selector as comma-separated key=value pairs
  nodeSelector: z.string().optional(),
  // Isolation settings
  runtimeClass: z.string().optional(),
  allowInternet: z.boolean().optional(),
  allowedEgressCidrs: z.string().optional(),
  allowedIngressPorts: z.string().optional(),
  allowClusterInternal: z.boolean().optional(),
  runAsUser: z.string().optional(),
  runAsGroup: z.string().optional(),
  runAsNonRoot: z.boolean().optional(),
  readOnlyRootFilesystem: z.boolean().optional(),
  allowPrivilegeEscalation: z.boolean().optional(),
  // Pool settings
  poolEnabled: z.boolean().optional(),
  poolSize: z.number().optional(),
})

type CreateEnvFormData = z.infer<typeof createEnvSchema>

// Helper to parse node selector string to object
function parseNodeSelector(input: string | undefined): Record<string, string> | undefined {
  if (!input?.trim()) return undefined
  const result: Record<string, string> = {}
  input.split(',').forEach(pair => {
    const [key, value] = pair.split('=').map(s => s.trim())
    if (key && value) {
      result[key] = value
    }
  })
  return Object.keys(result).length > 0 ? result : undefined
}

// Helper to parse comma-separated ports
function parsePorts(input: string | undefined): number[] | undefined {
  if (!input?.trim()) return undefined
  const ports = input.split(',')
    .map(s => parseInt(s.trim(), 10))
    .filter(n => !isNaN(n) && n > 0 && n <= 65535)
  return ports.length > 0 ? ports : undefined
}

// Helper to parse comma-separated CIDRs
function parseCidrs(input: string | undefined): string[] | undefined {
  if (!input?.trim()) return undefined
  const cidrs = input.split(',').map(s => s.trim()).filter(Boolean)
  return cidrs.length > 0 ? cidrs : undefined
}

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
  const [tolerations, setTolerations] = useState<TolerationEntry[]>([])
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
    mutationFn: (data: CreateEnvironmentData) => environmentsAPI.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['environments'] })
      setOpen(false)
      setTolerations([])
      reset()
    },
  })

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
    control,
  } = useForm<CreateEnvFormData>({
    resolver: zodResolver(createEnvSchema),
    defaultValues: {
      cpu: '500m',
      memory: '512Mi',
      storage: '1Gi',
      timeout: 3600,
      runtimeClass: '',
      allowInternet: false,
      allowClusterInternal: false,
      runAsNonRoot: false,
      readOnlyRootFilesystem: false,
      allowPrivilegeEscalation: false,
      poolEnabled: false,
      poolSize: 2,
    },
  })

  const onSubmit = (formData: CreateEnvFormData) => {
    // Build isolation config if any settings are provided
    let isolation: IsolationConfig | undefined = undefined
    
    const hasRuntimeClass = !!formData.runtimeClass
    const hasNetworkPolicy = formData.allowInternet || 
                             formData.allowClusterInternal || 
                             !!formData.allowedEgressCidrs || 
                             !!formData.allowedIngressPorts
    const hasSecurityContext = formData.runAsNonRoot || 
                                formData.readOnlyRootFilesystem || 
                                formData.allowPrivilegeEscalation === false ||
                                !!formData.runAsUser || 
                                !!formData.runAsGroup

    if (hasRuntimeClass || hasNetworkPolicy || hasSecurityContext) {
      isolation = {}
      
      if (hasRuntimeClass) {
        isolation.runtime_class = formData.runtimeClass
      }
      
      if (hasNetworkPolicy) {
        isolation.network_policy = {
          allow_internet: formData.allowInternet,
          allow_cluster_internal: formData.allowClusterInternal,
          allowed_egress_cidrs: parseCidrs(formData.allowedEgressCidrs),
          allowed_ingress_ports: parsePorts(formData.allowedIngressPorts),
        }
      }
      
      if (hasSecurityContext) {
        isolation.security_context = {
          run_as_non_root: formData.runAsNonRoot || undefined,
          read_only_root_filesystem: formData.readOnlyRootFilesystem || undefined,
          allow_privilege_escalation: formData.allowPrivilegeEscalation,
          run_as_user: formData.runAsUser ? parseInt(formData.runAsUser, 10) : undefined,
          run_as_group: formData.runAsGroup ? parseInt(formData.runAsGroup, 10) : undefined,
        }
      }
    }

    // Convert tolerations state to API format
    const tolerationsData: Toleration[] | undefined = tolerations.length > 0
      ? tolerations.filter(t => t.key).map(t => ({
          key: t.key,
          operator: t.operator,
          value: t.operator === 'Equal' ? t.value : undefined,
          effect: t.effect || undefined,
        }))
      : undefined

    // Build pool config if enabled
    let pool: PoolConfig | undefined = undefined
    if (formData.poolEnabled) {
      pool = {
        enabled: true,
        size: formData.poolSize || 2,
      }
    }

    createMutation.mutate({
      name: formData.name,
      image: formData.image,
      resources: {
        cpu: formData.cpu,
        memory: formData.memory,
        storage: formData.storage,
      },
      timeout: formData.timeout,
      node_selector: parseNodeSelector(formData.nodeSelector),
      tolerations: tolerationsData,
      isolation,
      pool,
    })
  }

  // Toleration management functions
  const addToleration = () => {
    setTolerations([...tolerations, { key: '', operator: 'Equal', value: '', effect: '' }])
  }

  const removeToleration = (index: number) => {
    setTolerations(tolerations.filter((_, i) => i !== index))
  }

  const updateToleration = (index: number, field: keyof TolerationEntry, value: string) => {
    const newTolerations = [...tolerations]
    newTolerations[index] = { ...newTolerations[index], [field]: value }
    setTolerations(newTolerations)
  }

  const handleDelete = (id: string) => {
    if (window.confirm('Are you sure you want to delete this environment?')) {
      deleteMutation.mutate(id)
    }
  }

  const envList = data?.environments as Environment[] | undefined

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
            ) : envList?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  No environments found
                </TableCell>
              </TableRow>
            ) : (
              envList?.map((env) => (
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

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="md" fullWidth>
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
            
            {/* Basic Settings */}
            <Typography variant="subtitle2" sx={{ mt: 1, mb: 1 }}>Basic Settings</Typography>
            <Grid container spacing={2}>
              <Grid item xs={12} sm={6}>
                <TextField
                  margin="dense"
                  label="Name"
                  fullWidth
                  required
                  {...register('name')}
                  error={!!errors.name}
                  helperText={errors.name?.message || 'Lowercase alphanumeric with hyphens'}
                />
              </Grid>
              <Grid item xs={12} sm={6}>
                <TextField
                  margin="dense"
                  label="Image"
                  fullWidth
                  required
                  placeholder="python:3.11-slim"
                  {...register('image')}
                  error={!!errors.image}
                  helperText={errors.image?.message}
                />
              </Grid>
            </Grid>

            {/* Resource Settings */}
            <Typography variant="subtitle2" sx={{ mt: 2, mb: 1 }}>Resources</Typography>
            <Grid container spacing={2}>
              <Grid item xs={12} sm={4}>
                <TextField
                  margin="dense"
                  label="CPU"
                  fullWidth
                  required
                  placeholder="500m"
                  {...register('cpu')}
                  error={!!errors.cpu}
                  helperText={errors.cpu?.message || 'e.g., 500m, 1, 2'}
                />
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField
                  margin="dense"
                  label="Memory"
                  fullWidth
                  required
                  placeholder="512Mi"
                  {...register('memory')}
                  error={!!errors.memory}
                  helperText={errors.memory?.message || 'e.g., 512Mi, 1Gi'}
                />
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField
                  margin="dense"
                  label="Storage"
                  fullWidth
                  required
                  placeholder="1Gi"
                  {...register('storage')}
                  error={!!errors.storage}
                  helperText={errors.storage?.message || 'e.g., 1Gi, 5Gi'}
                />
              </Grid>
            </Grid>

            {/* Advanced Settings Accordions */}
            <Divider sx={{ my: 2 }} />
            <Typography variant="subtitle2" sx={{ mb: 1 }}>Advanced Settings (Optional)</Typography>

            {/* Node Scheduling */}
            <Accordion>
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Typography>Node Scheduling</Typography>
                <Tooltip title="Control which nodes the environment runs on">
                  <InfoIcon fontSize="small" sx={{ ml: 1, color: 'text.secondary' }} />
                </Tooltip>
              </AccordionSummary>
              <AccordionDetails>
                <TextField
                  margin="dense"
                  label="Node Selector"
                  fullWidth
                  placeholder="kubernetes.io/arch=amd64,node-type=compute"
                  {...register('nodeSelector')}
                  helperText="Comma-separated key=value pairs (e.g., node-type=gpu,zone=us-east-1)"
                  sx={{ mb: 2 }}
                />
                
                <Typography variant="subtitle2" sx={{ mt: 2, mb: 1 }}>Tolerations</Typography>
                <Typography variant="caption" color="text.secondary" sx={{ mb: 2, display: 'block' }}>
                  Allow scheduling on nodes with matching taints
                </Typography>
                
                {tolerations.map((tol, index) => (
                  <Box key={index} sx={{ display: 'flex', gap: 1, mb: 1, alignItems: 'center' }}>
                    <TextField
                      size="small"
                      label="Key"
                      value={tol.key}
                      onChange={(e) => updateToleration(index, 'key', e.target.value)}
                      sx={{ flex: 2 }}
                      placeholder="dedicated"
                    />
                    <FormControl size="small" sx={{ flex: 1 }}>
                      <InputLabel>Operator</InputLabel>
                      <Select
                        value={tol.operator}
                        label="Operator"
                        onChange={(e) => updateToleration(index, 'operator', e.target.value)}
                      >
                        <MenuItem value="Equal">Equal</MenuItem>
                        <MenuItem value="Exists">Exists</MenuItem>
                      </Select>
                    </FormControl>
                    <TextField
                      size="small"
                      label="Value"
                      value={tol.value}
                      onChange={(e) => updateToleration(index, 'value', e.target.value)}
                      disabled={tol.operator === 'Exists'}
                      sx={{ flex: 2 }}
                      placeholder="agents"
                    />
                    <FormControl size="small" sx={{ flex: 1.5 }}>
                      <InputLabel>Effect</InputLabel>
                      <Select
                        value={tol.effect}
                        label="Effect"
                        onChange={(e) => updateToleration(index, 'effect', e.target.value)}
                      >
                        <MenuItem value="">Any</MenuItem>
                        <MenuItem value="NoSchedule">NoSchedule</MenuItem>
                        <MenuItem value="PreferNoSchedule">PreferNoSchedule</MenuItem>
                        <MenuItem value="NoExecute">NoExecute</MenuItem>
                      </Select>
                    </FormControl>
                    <IconButton size="small" onClick={() => removeToleration(index)} color="error">
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </Box>
                ))}
                
                <Button
                  size="small"
                  startIcon={<AddIcon />}
                  onClick={addToleration}
                  sx={{ mt: 1 }}
                >
                  Add Toleration
                </Button>
              </AccordionDetails>
            </Accordion>

            {/* Runtime Isolation */}
            <Accordion>
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Typography>Runtime Isolation</Typography>
                <Tooltip title="Container runtime and sandbox settings">
                  <InfoIcon fontSize="small" sx={{ ml: 1, color: 'text.secondary' }} />
                </Tooltip>
              </AccordionSummary>
              <AccordionDetails>
                <FormControl fullWidth margin="dense">
                  <InputLabel>Runtime Class</InputLabel>
                  <Controller
                    name="runtimeClass"
                    control={control}
                    render={({ field }) => (
                      <Select {...field} label="Runtime Class">
                        <MenuItem value="">Default (cluster default)</MenuItem>
                        <MenuItem value="gvisor">gVisor (strong isolation)</MenuItem>
                        <MenuItem value="kata-qemu">Kata Containers (VM-based)</MenuItem>
                        <MenuItem value="runc">runc (standard OCI)</MenuItem>
                      </Select>
                    )}
                  />
                </FormControl>
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
                  gVisor provides application-kernel isolation. Kata runs containers in lightweight VMs.
                </Typography>
              </AccordionDetails>
            </Accordion>

            {/* Network Policy */}
            <Accordion>
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Typography>Network Policy</Typography>
                <Tooltip title="Control network access for the environment">
                  <InfoIcon fontSize="small" sx={{ ml: 1, color: 'text.secondary' }} />
                </Tooltip>
              </AccordionSummary>
              <AccordionDetails>
                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <Controller
                      name="allowInternet"
                      control={control}
                      render={({ field }) => (
                        <FormControlLabel
                          control={<Switch checked={field.value} onChange={field.onChange} />}
                          label="Allow Internet Access"
                        />
                      )}
                    />
                    <Typography variant="caption" color="text.secondary" display="block">
                      Enable full outbound internet connectivity
                    </Typography>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Controller
                      name="allowClusterInternal"
                      control={control}
                      render={({ field }) => (
                        <FormControlLabel
                          control={<Switch checked={field.value} onChange={field.onChange} />}
                          label="Allow Cluster Internal"
                        />
                      )}
                    />
                    <Typography variant="caption" color="text.secondary" display="block">
                      Allow traffic to/from other pods in cluster
                    </Typography>
                  </Grid>
                </Grid>
                <TextField
                  margin="dense"
                  label="Allowed Egress CIDRs"
                  fullWidth
                  placeholder="10.0.0.0/8,192.168.0.0/16"
                  {...register('allowedEgressCidrs')}
                  helperText="Comma-separated IP ranges for outbound traffic (e.g., 10.0.0.0/8)"
                  sx={{ mt: 2 }}
                />
                <TextField
                  margin="dense"
                  label="Allowed Ingress Ports"
                  fullWidth
                  placeholder="8080,443,3000"
                  {...register('allowedIngressPorts')}
                  helperText="Comma-separated ports to allow inbound traffic"
                />
              </AccordionDetails>
            </Accordion>

            {/* Security Context */}
            <Accordion>
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Typography>Security Context</Typography>
                <Tooltip title="Pod security settings for defense in depth">
                  <InfoIcon fontSize="small" sx={{ ml: 1, color: 'text.secondary' }} />
                </Tooltip>
              </AccordionSummary>
              <AccordionDetails>
                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <TextField
                      margin="dense"
                      label="Run as User (UID)"
                      fullWidth
                      type="number"
                      placeholder="1000"
                      {...register('runAsUser')}
                      helperText="UID to run the container as"
                    />
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <TextField
                      margin="dense"
                      label="Run as Group (GID)"
                      fullWidth
                      type="number"
                      placeholder="1000"
                      {...register('runAsGroup')}
                      helperText="GID to run the container as"
                    />
                  </Grid>
                </Grid>
                <Box sx={{ mt: 2 }}>
                  <Controller
                    name="runAsNonRoot"
                    control={control}
                    render={({ field }) => (
                      <FormControlLabel
                        control={<Switch checked={field.value} onChange={field.onChange} />}
                        label="Run as Non-Root"
                      />
                    )}
                  />
                  <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 4 }}>
                    Enforce running as non-root user (recommended)
                  </Typography>
                </Box>
                <Box sx={{ mt: 1 }}>
                  <Controller
                    name="readOnlyRootFilesystem"
                    control={control}
                    render={({ field }) => (
                      <FormControlLabel
                        control={<Switch checked={field.value} onChange={field.onChange} />}
                        label="Read-Only Root Filesystem"
                      />
                    )}
                  />
                  <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 4 }}>
                    Mount root filesystem as read-only (use volumes for writable data)
                  </Typography>
                </Box>
                <Box sx={{ mt: 1 }}>
                  <Controller
                    name="allowPrivilegeEscalation"
                    control={control}
                    render={({ field }) => (
                      <FormControlLabel
                        control={<Switch checked={field.value} onChange={field.onChange} />}
                        label="Allow Privilege Escalation"
                      />
                    )}
                  />
                  <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 4 }}>
                    Allow processes to gain more privileges (not recommended)
                  </Typography>
                </Box>
              </AccordionDetails>
            </Accordion>

            {/* Standby Pod Pool */}
            <Accordion>
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Typography>Standby Pod Pool</Typography>
                <Tooltip title="Pre-warm pods for faster command execution">
                  <InfoIcon fontSize="small" sx={{ ml: 1, color: 'text.secondary' }} />
                </Tooltip>
              </AccordionSummary>
              <AccordionDetails>
                <Alert severity="info" sx={{ mb: 2 }}>
                  Standby pods are pre-warmed and ready to execute commands immediately, reducing startup latency from ~2-3 seconds to ~100ms.
                </Alert>
                <Box>
                  <Controller
                    name="poolEnabled"
                    control={control}
                    render={({ field }) => (
                      <FormControlLabel
                        control={<Switch checked={field.value} onChange={field.onChange} />}
                        label="Enable Standby Pool"
                      />
                    )}
                  />
                  <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 4 }}>
                    Maintain pre-warmed pods using this environment's image
                  </Typography>
                </Box>
                <Controller
                  name="poolSize"
                  control={control}
                  render={({ field }) => (
                    <TextField
                      margin="dense"
                      label="Pool Size"
                      fullWidth
                      type="number"
                      value={field.value}
                      onChange={(e) => field.onChange(parseInt(e.target.value, 10) || 2)}
                      helperText="Number of standby pods to maintain (default: 2)"
                      sx={{ mt: 2 }}
                      inputProps={{ min: 1, max: 10 }}
                    />
                  )}
                />
              </AccordionDetails>
            </Accordion>

          </DialogContent>
          <DialogActions>
            <Button onClick={() => { setOpen(false); reset(); setTolerations([]) }}>Cancel</Button>
            <Button type="submit" variant="contained" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </Box>
  )
}
