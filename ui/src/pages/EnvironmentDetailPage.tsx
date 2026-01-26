import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  Box,
  Paper,
  Typography,
  Tabs,
  Tab,
  Button,
  CircularProgress,
  Alert,
  Chip,
  Divider,
  Grid,
  Table,
  TableBody,
  TableCell,
  TableRow,
  Accordion,
  AccordionSummary,
  AccordionDetails,
} from '@mui/material'
import { 
  ArrowBack as ArrowBackIcon,
  ExpandMore as ExpandMoreIcon,
  Check as CheckIcon,
  Close as CloseIcon,
} from '@mui/icons-material'
import { environmentsAPI } from '../services/api'
import TerminalView from '../components/common/TerminalView'
import LogViewer from '../components/common/LogViewer'
import ExecutionPanel from '../components/common/ExecutionPanel'

interface TabPanelProps {
  children?: React.ReactNode
  index: number
  value: number
}

function TabPanel(props: TabPanelProps) {
  const { children, value, index, ...other } = props
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`tabpanel-${index}`}
      aria-labelledby={`tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{ p: 3 }}>{children}</Box>}
    </div>
  )
}

export default function EnvironmentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [tab, setTab] = useState(0)

  const { data: environment, isLoading, error } = useQuery({
    queryKey: ['environment', id],
    queryFn: () => environmentsAPI.get(id!),
    enabled: !!id,
  })

  if (isLoading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
        <CircularProgress />
      </Box>
    )
  }

  if (error || !environment) {
    return (
      <Alert severity="error">
        Failed to load environment. {error instanceof Error ? error.message : ''}
      </Alert>
    )
  }

  return (
    <Box>
      <Button
        startIcon={<ArrowBackIcon />}
        onClick={() => navigate('/environments')}
        sx={{ mb: 2 }}
      >
        Back to Environments
      </Button>
      <Typography variant="h4" gutterBottom>
        {environment.name}
      </Typography>
      <Paper sx={{ mt: 2 }}>
        <Tabs value={tab} onChange={(_, newValue) => setTab(newValue)}>
          <Tab label="Overview" />
          <Tab label="Executions" />
          <Tab label="Terminal" />
          <Tab label="Logs" />
        </Tabs>
        <TabPanel value={tab} index={0}>
          <Box>
            {/* Basic Information */}
            <Typography variant="h6" gutterBottom>
              Environment Details
            </Typography>
            <Grid container spacing={3}>
              <Grid item xs={12} md={6}>
                <Table size="small">
                  <TableBody>
                    <TableRow>
                      <TableCell><strong>ID</strong></TableCell>
                      <TableCell>{environment.id}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell><strong>Status</strong></TableCell>
                      <TableCell>
                        <Chip 
                          label={environment.status} 
                          color={environment.status === 'running' ? 'success' : environment.status === 'pending' ? 'warning' : 'default'}
                          size="small"
                        />
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell><strong>Image</strong></TableCell>
                      <TableCell><code>{environment.image}</code></TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell><strong>Created</strong></TableCell>
                      <TableCell>{new Date(environment.created_at).toLocaleString()}</TableCell>
                    </TableRow>
                    {environment.started_at && (
                      <TableRow>
                        <TableCell><strong>Started</strong></TableCell>
                        <TableCell>{new Date(environment.started_at).toLocaleString()}</TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </Grid>
              <Grid item xs={12} md={6}>
                <Table size="small">
                  <TableBody>
                    <TableRow>
                      <TableCell><strong>CPU</strong></TableCell>
                      <TableCell>{environment.resources?.cpu}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell><strong>Memory</strong></TableCell>
                      <TableCell>{environment.resources?.memory}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell><strong>Storage</strong></TableCell>
                      <TableCell>{environment.resources?.storage}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </Grid>
            </Grid>

            <Divider sx={{ my: 3 }} />

            {/* Node Scheduling */}
            {(environment.node_selector || environment.tolerations) && (
              <Accordion defaultExpanded={false}>
                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                  <Typography variant="subtitle1">Node Scheduling</Typography>
                </AccordionSummary>
                <AccordionDetails>
                  {environment.node_selector && Object.keys(environment.node_selector).length > 0 && (
                    <Box sx={{ mb: 2 }}>
                      <Typography variant="subtitle2" gutterBottom>Node Selector</Typography>
                      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
                        {Object.entries(environment.node_selector).map(([key, value]) => (
                          <Chip key={key} label={`${key}=${value}`} size="small" variant="outlined" />
                        ))}
                      </Box>
                    </Box>
                  )}
                  {environment.tolerations && environment.tolerations.length > 0 && (
                    <Box>
                      <Typography variant="subtitle2" gutterBottom>Tolerations</Typography>
                      <Table size="small">
                        <TableBody>
                          {environment.tolerations.map((tol: { key?: string; operator?: string; value?: string; effect?: string; tolerationSeconds?: number }, idx: number) => (
                            <TableRow key={idx}>
                              <TableCell>{tol.key || '*'}</TableCell>
                              <TableCell>{tol.operator}</TableCell>
                              <TableCell>{tol.value || '-'}</TableCell>
                              <TableCell>{tol.effect}</TableCell>
                              {tol.tolerationSeconds && <TableCell>{tol.tolerationSeconds}s</TableCell>}
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </Box>
                  )}
                </AccordionDetails>
              </Accordion>
            )}

            {/* Isolation Settings */}
            {environment.isolation && (
              <Accordion defaultExpanded={false}>
                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                  <Typography variant="subtitle1">Isolation Settings</Typography>
                </AccordionSummary>
                <AccordionDetails>
                  <Grid container spacing={3}>
                    {/* Runtime Class */}
                    {environment.isolation.runtime_class && (
                      <Grid item xs={12}>
                        <Typography variant="subtitle2">Runtime Class</Typography>
                        <Chip 
                          label={environment.isolation.runtime_class} 
                          color="primary" 
                          variant="outlined"
                          size="small"
                        />
                      </Grid>
                    )}

                    {/* Network Policy */}
                    {environment.isolation.network_policy && (
                      <Grid item xs={12} md={6}>
                        <Typography variant="subtitle2" gutterBottom>Network Policy</Typography>
                        <Table size="small">
                          <TableBody>
                            <TableRow>
                              <TableCell>Internet Access</TableCell>
                              <TableCell>
                                {environment.isolation.network_policy.allow_internet ? 
                                  <CheckIcon color="success" fontSize="small" /> : 
                                  <CloseIcon color="error" fontSize="small" />}
                              </TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell>Cluster Internal</TableCell>
                              <TableCell>
                                {environment.isolation.network_policy.allow_cluster_internal ? 
                                  <CheckIcon color="success" fontSize="small" /> : 
                                  <CloseIcon color="error" fontSize="small" />}
                              </TableCell>
                            </TableRow>
                            {environment.isolation.network_policy.allowed_egress_cidrs && 
                             environment.isolation.network_policy.allowed_egress_cidrs.length > 0 && (
                              <TableRow>
                                <TableCell>Allowed Egress CIDRs</TableCell>
                                <TableCell>
                                  {environment.isolation.network_policy.allowed_egress_cidrs.join(', ')}
                                </TableCell>
                              </TableRow>
                            )}
                            {environment.isolation.network_policy.allowed_ingress_ports && 
                             environment.isolation.network_policy.allowed_ingress_ports.length > 0 && (
                              <TableRow>
                                <TableCell>Allowed Ingress Ports</TableCell>
                                <TableCell>
                                  {environment.isolation.network_policy.allowed_ingress_ports.join(', ')}
                                </TableCell>
                              </TableRow>
                            )}
                          </TableBody>
                        </Table>
                      </Grid>
                    )}

                    {/* Security Context */}
                    {environment.isolation.security_context && (
                      <Grid item xs={12} md={6}>
                        <Typography variant="subtitle2" gutterBottom>Security Context</Typography>
                        <Table size="small">
                          <TableBody>
                            {environment.isolation.security_context.run_as_user !== undefined && (
                              <TableRow>
                                <TableCell>Run as User</TableCell>
                                <TableCell>{environment.isolation.security_context.run_as_user}</TableCell>
                              </TableRow>
                            )}
                            {environment.isolation.security_context.run_as_group !== undefined && (
                              <TableRow>
                                <TableCell>Run as Group</TableCell>
                                <TableCell>{environment.isolation.security_context.run_as_group}</TableCell>
                              </TableRow>
                            )}
                            <TableRow>
                              <TableCell>Run as Non-Root</TableCell>
                              <TableCell>
                                {environment.isolation.security_context.run_as_non_root ? 
                                  <CheckIcon color="success" fontSize="small" /> : 
                                  <CloseIcon color="error" fontSize="small" />}
                              </TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell>Read-Only Root FS</TableCell>
                              <TableCell>
                                {environment.isolation.security_context.read_only_root_filesystem ? 
                                  <CheckIcon color="success" fontSize="small" /> : 
                                  <CloseIcon color="error" fontSize="small" />}
                              </TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell>Allow Privilege Escalation</TableCell>
                              <TableCell>
                                {environment.isolation.security_context.allow_privilege_escalation ? 
                                  <CheckIcon color="warning" fontSize="small" /> : 
                                  <CloseIcon color="success" fontSize="small" />}
                              </TableCell>
                            </TableRow>
                          </TableBody>
                        </Table>
                      </Grid>
                    )}
                  </Grid>
                </AccordionDetails>
              </Accordion>
            )}

            {/* Pool Settings */}
            {environment.pool && environment.pool.enabled && (
              <Accordion defaultExpanded={false}>
                <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                  <Typography variant="subtitle1">Standby Pod Pool</Typography>
                </AccordionSummary>
                <AccordionDetails>
                  <Table size="small">
                    <TableBody>
                      <TableRow>
                        <TableCell><strong>Status</strong></TableCell>
                        <TableCell>
                          <Chip label="Enabled" color="success" size="small" />
                        </TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell><strong>Pool Size</strong></TableCell>
                        <TableCell>{environment.pool.size || 2} standby pods</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                  <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
                    Pre-warmed pods are maintained for faster command execution (~100ms vs ~2-3s)
                  </Typography>
                </AccordionDetails>
              </Accordion>
            )}
          </Box>
        </TabPanel>
        <TabPanel value={tab} index={1}>
          <ExecutionPanel environmentId={id!} />
        </TabPanel>
        <TabPanel value={tab} index={2}>
          <TerminalView environmentId={id!} />
        </TabPanel>
        <TabPanel value={tab} index={3}>
          <LogViewer environmentId={id!} />
        </TabPanel>
      </Paper>
    </Box>
  )
}
