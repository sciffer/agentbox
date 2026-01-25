import { useEffect, useRef, useState, useCallback } from 'react'
import { Box, Typography, Button, TextField, FormControlLabel, Switch } from '@mui/material'
import { Refresh as RefreshIcon } from '@mui/icons-material'
import { environmentsAPI } from '../../services/api'
import { LogEntry } from '../../types'

interface LogViewerProps {
  environmentId: string
}

export default function LogViewer({ environmentId }: LogViewerProps) {
  const [logs, setLogs] = useState<string[]>([])
  const [follow, setFollow] = useState(false)
  const [tail, setTail] = useState(100)
  const [includeTimestamps, setIncludeTimestamps] = useState(true)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const eventSourceRef = useRef<EventSource | null>(null)

  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => {
    scrollToBottom()
  }, [logs])

  const fetchLogs = useCallback(async () => {
    try {
      const response = await environmentsAPI.getLogs(environmentId, {
        tail,
        timestamps: includeTimestamps,
      })
      setLogs(response.logs?.map((log: LogEntry) => log.message) || [])
    } catch (error) {
      console.error('Failed to fetch logs:', error)
    }
  }, [environmentId, tail, includeTimestamps])

  useEffect(() => {
    if (follow) {
      const url = `http://${window.location.hostname}:8080/api/v1/environments/${environmentId}/logs?follow=true&timestamps=${includeTimestamps}`
      const eventSource = new EventSource(url)

      eventSource.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as LogEntry
          setLogs((prev) => [...prev, data.message || event.data])
        } catch {
          setLogs((prev) => [...prev, event.data])
        }
      }

      eventSource.onerror = () => {
        eventSource.close()
      }

      eventSourceRef.current = eventSource

      return () => {
        eventSource.close()
      }
    } else {
      fetchLogs()
    }
  }, [environmentId, follow, includeTimestamps, fetchLogs])

  return (
    <Box>
      <Box display="flex" gap={2} mb={2} alignItems="center">
        <TextField
          label="Tail Lines"
          type="number"
          value={tail}
          onChange={(e) => setTail(parseInt(e.target.value) || 100)}
          size="small"
          sx={{ width: 150 }}
        />
        <FormControlLabel
          control={
            <Switch
              checked={includeTimestamps}
              onChange={(e) => setIncludeTimestamps(e.target.checked)}
            />
          }
          label="Include Timestamps"
        />
        <FormControlLabel
          control={
            <Switch
              checked={follow}
              onChange={(e) => setFollow(e.target.checked)}
            />
          }
          label="Follow"
        />
        <Button
          startIcon={<RefreshIcon />}
          onClick={fetchLogs}
          disabled={follow}
        >
          Refresh
        </Button>
      </Box>
      <Box
        sx={{
          backgroundColor: '#1e1e1e',
          color: '#d4d4d4',
          fontFamily: 'monospace',
          fontSize: '14px',
          padding: 2,
          borderRadius: 1,
          height: '500px',
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}
      >
        {logs.length === 0 ? (
          <Typography color="text.secondary">No logs available</Typography>
        ) : (
          logs.map((log, index) => (
            <div key={index}>{log}</div>
          ))
        )}
        <div ref={logsEndRef} />
      </Box>
    </Box>
  )
}
