import { useEffect, useRef } from 'react'
import { Box, Typography } from '@mui/material'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'

interface TerminalViewProps {
  environmentId: string
}

export default function TerminalView({ environmentId }: TerminalViewProps) {
  const terminalRef = useRef<HTMLDivElement>(null)
  const terminalInstanceRef = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!terminalRef.current) return

    const terminal = new Terminal({
      cursorBlink: true,
      theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
      },
    })

    const fitAddon = new FitAddon()
    terminal.loadAddon(fitAddon)
    terminal.open(terminalRef.current)
    fitAddon.fit()

    terminalInstanceRef.current = terminal

    // Connect to WebSocket
    const wsUrl = `ws://${window.location.hostname}:8080/api/v1/environments/${environmentId}/attach`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      terminal.write('\r\nConnected to environment terminal...\r\n')
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        if (message.type === 'stdout' || message.type === 'stderr') {
          terminal.write(message.data)
        }
      } catch (e) {
        terminal.write(event.data)
      }
    }

    ws.onerror = () => {
      terminal.write('\r\nError connecting to terminal\r\n')
    }

    ws.onclose = () => {
      terminal.write('\r\nTerminal connection closed\r\n')
    }

    terminal.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'stdin', data }))
      }
    })

    wsRef.current = ws

    const handleResize = () => {
      fitAddon.fit()
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      ws.close()
      terminal.dispose()
    }
  }, [environmentId])

  return (
    <Box>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        Interactive terminal for environment {environmentId}
      </Typography>
      <Box
        ref={terminalRef}
        sx={{
          width: '100%',
          height: '500px',
          backgroundColor: '#1e1e1e',
          borderRadius: 1,
          p: 1,
        }}
      />
    </Box>
  )
}
