import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Box,
  Grid,
  Paper,
  Typography,
  Card,
  CardContent,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Chip,
  CircularProgress,
  Alert,
} from '@mui/material'
import {
  Storage as StorageIcon,
  CheckCircle as CheckCircleIcon,
  Error as ErrorIcon,
  Schedule as ScheduleIcon,
  Memory as MemoryIcon,
  Speed as SpeedIcon,
} from '@mui/icons-material'
import { environmentsAPI, metricsAPI } from '../services/api'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, AreaChart, Area } from 'recharts'
import { Environment, Metric } from '../types'

export default function DashboardPage() {
  const [selectedEnvId, setSelectedEnvId] = useState<string>('all')
  const [timeRange, setTimeRange] = useState<string>('1h')

  // Fetch environments
  const { data: environments, isLoading: envsLoading } = useQuery({
    queryKey: ['environments'],
    queryFn: () => environmentsAPI.list(),
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  // Calculate time range
  const { startTime, endTime } = useMemo(() => {
    const end = new Date()
    let start = new Date()
    switch (timeRange) {
      case '15m':
        start = new Date(end.getTime() - 15 * 60 * 1000)
        break
      case '1h':
        start = new Date(end.getTime() - 60 * 60 * 1000)
        break
      case '6h':
        start = new Date(end.getTime() - 6 * 60 * 60 * 1000)
        break
      case '24h':
        start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
        break
      default:
        start = new Date(end.getTime() - 60 * 60 * 1000)
    }
    return { startTime: start.toISOString(), endTime: end.toISOString() }
  }, [timeRange])

  // Fetch CPU metrics
  const { data: cpuMetrics } = useQuery({
    queryKey: ['metrics', selectedEnvId, 'cpu_usage', startTime, endTime],
    queryFn: () => 
      selectedEnvId === 'all'
        ? metricsAPI.getGlobal({ type: 'cpu_usage', start: startTime, end: endTime })
        : metricsAPI.getEnvironment(selectedEnvId, { type: 'cpu_usage', start: startTime, end: endTime }),
    refetchInterval: 30000,
  })

  // Fetch memory metrics
  const { data: memoryMetrics } = useQuery({
    queryKey: ['metrics', selectedEnvId, 'memory_usage', startTime, endTime],
    queryFn: () => 
      selectedEnvId === 'all'
        ? metricsAPI.getGlobal({ type: 'memory_usage', start: startTime, end: endTime })
        : metricsAPI.getEnvironment(selectedEnvId, { type: 'memory_usage', start: startTime, end: endTime }),
    refetchInterval: 30000,
  })

  // Fetch running sandboxes metrics
  const { data: sandboxMetrics } = useQuery({
    queryKey: ['metrics', 'global', 'running_sandboxes', startTime, endTime],
    queryFn: () => metricsAPI.getGlobal({ type: 'running_sandboxes', start: startTime, end: endTime }),
    refetchInterval: 30000,
  })

  // Handle both direct array response and wrapped response {environments: [...], total: X}
  const envData = environments as { environments?: Environment[]; total?: number } | Environment[] | undefined
  const envList: Environment[] = Array.isArray(envData) 
    ? envData 
    : (envData?.environments || [])
  
  const runningEnvs = envList.filter((e) => e.status === 'running')

  // When a specific environment is selected, show stats for that env only; otherwise show all
  const listForStats = selectedEnvId !== 'all' && selectedEnvId
    ? envList.filter((e) => e.id === selectedEnvId)
    : envList
  const stats = {
    total: listForStats.length,
    running: listForStats.filter((e) => e.status === 'running').length,
    pending: listForStats.filter((e) => e.status === 'pending').length,
    failed: listForStats.filter((e) => e.status === 'failed').length,
  }

  // Format metrics for charts
  const cpuChartData = useMemo(() => {
    const metricsList = cpuMetrics?.metrics as Metric[] | undefined
    return metricsList?.map((m) => ({
      time: new Date(m.timestamp).toLocaleTimeString(),
      cpu: m.value,
    })) || []
  }, [cpuMetrics])

  const memoryChartData = useMemo(() => {
    const metricsList = memoryMetrics?.metrics as Metric[] | undefined
    return metricsList?.map((m) => ({
      time: new Date(m.timestamp).toLocaleTimeString(),
      memory: m.value,
    })) || []
  }, [memoryMetrics])

  const sandboxChartData = useMemo(() => {
    const metricsList = sandboxMetrics?.metrics as Metric[] | undefined
    return metricsList?.map((m) => ({
      time: new Date(m.timestamp).toLocaleTimeString(),
      value: m.value,
    })) || []
  }, [sandboxMetrics])

  // Get current values (latest metric)
  const currentCPU = cpuChartData.length > 0 ? cpuChartData[cpuChartData.length - 1].cpu : 0
  const currentMemory = memoryChartData.length > 0 ? memoryChartData[memoryChartData.length - 1].memory : 0

  const selectedEnv = selectedEnvId !== 'all' ? envList?.find(e => e.id === selectedEnvId) : null

  if (envsLoading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
        <CircularProgress />
      </Box>
    )
  }

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4">Dashboard</Typography>
        <Box display="flex" gap={2}>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <InputLabel>Environment</InputLabel>
            <Select
              value={selectedEnvId}
              label="Environment"
              onChange={(e) => setSelectedEnvId(e.target.value)}
            >
              <MenuItem value="all">
                <Box display="flex" alignItems="center" gap={1}>
                  All Environments
                  <Chip label={stats.running} size="small" color="primary" />
                </Box>
              </MenuItem>
              {runningEnvs.map((env) => (
                <MenuItem key={env.id} value={env.id}>
                  <Box display="flex" alignItems="center" gap={1}>
                    {env.name}
                    <Chip 
                      label={env.status} 
                      size="small" 
                      color={env.status === 'running' ? 'success' : 'default'} 
                    />
                  </Box>
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 120 }}>
            <InputLabel>Time Range</InputLabel>
            <Select
              value={timeRange}
              label="Time Range"
              onChange={(e) => setTimeRange(e.target.value)}
            >
              <MenuItem value="15m">Last 15 min</MenuItem>
              <MenuItem value="1h">Last 1 hour</MenuItem>
              <MenuItem value="6h">Last 6 hours</MenuItem>
              <MenuItem value="24h">Last 24 hours</MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Box>

      {selectedEnv && (
        <Alert severity="info" sx={{ mb: 2 }}>
          Showing metrics for: <strong>{selectedEnv.name}</strong> ({selectedEnv.id})
          <br />
          Image: {selectedEnv.image} | Resources: {selectedEnv.resources.cpu} CPU, {selectedEnv.resources.memory} Memory
        </Alert>
      )}

      <Grid container spacing={3}>
        {/* Summary Cards */}
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center">
                <StorageIcon color="primary" sx={{ mr: 2, fontSize: 40 }} />
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Total Environments
                  </Typography>
                  <Typography variant="h4">{stats.total}</Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center">
                <CheckCircleIcon color="success" sx={{ mr: 2, fontSize: 40 }} />
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Running
                  </Typography>
                  <Typography variant="h4">{stats.running}</Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center">
                <ScheduleIcon color="warning" sx={{ mr: 2, fontSize: 40 }} />
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Pending
                  </Typography>
                  <Typography variant="h4">{stats.pending}</Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center">
                <ErrorIcon color="error" sx={{ mr: 2, fontSize: 40 }} />
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Failed
                  </Typography>
                  <Typography variant="h4">{stats.failed}</Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* Current Resource Usage Cards */}
        <Grid item xs={12} sm={6}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center">
                <SpeedIcon color="info" sx={{ mr: 2, fontSize: 40 }} />
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Current CPU Usage
                  </Typography>
                  <Typography variant="h4">
                    {currentCPU.toFixed(0)} <Typography component="span" variant="h6">millicores</Typography>
                  </Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center">
                <MemoryIcon color="secondary" sx={{ mr: 2, fontSize: 40 }} />
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Current Memory Usage
                  </Typography>
                  <Typography variant="h4">
                    {currentMemory.toFixed(1)} <Typography component="span" variant="h6">MiB</Typography>
                  </Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* CPU Usage Chart */}
        <Grid item xs={12} md={6}>
          <Paper sx={{ p: 3 }}>
            <Typography variant="h6" gutterBottom>
              CPU Usage Over Time {selectedEnvId !== 'all' && `(${selectedEnv?.name || selectedEnvId})`}
            </Typography>
            {cpuChartData.length > 0 ? (
              <ResponsiveContainer width="100%" height={250}>
                <AreaChart data={cpuChartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="time" tick={{ fontSize: 12 }} />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip 
                    formatter={(value: number) => [`${value.toFixed(0)} millicores`, 'CPU']}
                  />
                  <Legend />
                  <Area 
                    type="monotone" 
                    dataKey="cpu" 
                    stroke="#2196f3" 
                    fill="#2196f3" 
                    fillOpacity={0.3}
                    name="CPU (millicores)" 
                  />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <Box display="flex" justifyContent="center" alignItems="center" height={250}>
                <Typography color="textSecondary">No CPU metrics available</Typography>
              </Box>
            )}
          </Paper>
        </Grid>

        {/* Memory Usage Chart */}
        <Grid item xs={12} md={6}>
          <Paper sx={{ p: 3 }}>
            <Typography variant="h6" gutterBottom>
              Memory Usage Over Time {selectedEnvId !== 'all' && `(${selectedEnv?.name || selectedEnvId})`}
            </Typography>
            {memoryChartData.length > 0 ? (
              <ResponsiveContainer width="100%" height={250}>
                <AreaChart data={memoryChartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="time" tick={{ fontSize: 12 }} />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip 
                    formatter={(value: number) => [`${value.toFixed(1)} MiB`, 'Memory']}
                  />
                  <Legend />
                  <Area 
                    type="monotone" 
                    dataKey="memory" 
                    stroke="#9c27b0" 
                    fill="#9c27b0" 
                    fillOpacity={0.3}
                    name="Memory (MiB)" 
                  />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <Box display="flex" justifyContent="center" alignItems="center" height={250}>
                <Typography color="textSecondary">No memory metrics available</Typography>
              </Box>
            )}
          </Paper>
        </Grid>

        {/* Running Sandboxes Chart (Global only) */}
        <Grid item xs={12}>
          <Paper sx={{ p: 3 }}>
            <Typography variant="h6" gutterBottom>
              Running Sandboxes Over Time
            </Typography>
            {sandboxChartData.length > 0 ? (
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={sandboxChartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="time" tick={{ fontSize: 12 }} />
                  <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                  <Tooltip />
                  <Legend />
                  <Line 
                    type="monotone" 
                    dataKey="value" 
                    stroke="#4caf50" 
                    strokeWidth={2}
                    dot={false}
                    name="Running Sandboxes" 
                  />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <Box display="flex" justifyContent="center" alignItems="center" height={300}>
                <Typography color="textSecondary">No sandbox metrics available</Typography>
              </Box>
            )}
          </Paper>
        </Grid>
      </Grid>
    </Box>
  )
}
