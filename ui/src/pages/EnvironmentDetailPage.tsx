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
} from '@mui/material'
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material'
import { environmentsAPI } from '../services/api'
import TerminalView from '../components/common/TerminalView'
import LogViewer from '../components/common/LogViewer'

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
          <Tab label="Terminal" />
          <Tab label="Logs" />
        </Tabs>
        <TabPanel value={tab} index={0}>
          <Box>
            <Typography variant="h6" gutterBottom>
              Environment Details
            </Typography>
            <Typography><strong>ID:</strong> {environment.id}</Typography>
            <Typography><strong>Status:</strong> {environment.status}</Typography>
            <Typography><strong>Image:</strong> {environment.image}</Typography>
            <Typography><strong>CPU:</strong> {environment.resources?.cpu}</Typography>
            <Typography><strong>Memory:</strong> {environment.resources?.memory}</Typography>
            <Typography><strong>Storage:</strong> {environment.resources?.storage}</Typography>
            <Typography><strong>Created:</strong> {new Date(environment.created_at).toLocaleString()}</Typography>
          </Box>
        </TabPanel>
        <TabPanel value={tab} index={1}>
          <TerminalView environmentId={id!} />
        </TabPanel>
        <TabPanel value={tab} index={2}>
          <LogViewer environmentId={id!} />
        </TabPanel>
      </Paper>
    </Box>
  )
}
