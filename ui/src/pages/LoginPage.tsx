import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Container,
  Box,
  Paper,
  TextField,
  Button,
  Typography,
  Alert,
  Tab,
  Tabs,
} from '@mui/material'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { AxiosError } from 'axios'
import { authAPI } from '../services/api'
import { useAuthStore } from '../store/authStore'

const loginSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
})

type LoginFormData = z.infer<typeof loginSchema>

interface APIErrorResponse {
  message?: string
}

export default function LoginPage() {
  const [tab, setTab] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const { setAuth } = useAuthStore()

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
  })

  const onSubmit = async (data: LoginFormData) => {
    setError(null)
    setLoading(true)
    try {
      const response = await authAPI.login(data.username, data.password)
      setAuth(response.token, response.user)
      navigate('/')
    } catch (err) {
      const axiosError = err as AxiosError<APIErrorResponse>
      setError(axiosError.response?.data?.message || 'Login failed. Please check your credentials.')
    } finally {
      setLoading(false)
    }
  }

  const handleGoogleLogin = () => {
    // TODO: Implement Google OAuth
    setError('Google OAuth not yet implemented')
  }

  return (
    <Container component="main" maxWidth="xs">
      <Box
        sx={{
          marginTop: 8,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
        }}
      >
        <Typography component="h1" variant="h4" gutterBottom>
          AgentBox
        </Typography>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          Sandbox Management Platform
        </Typography>
        <Paper elevation={3} sx={{ mt: 3, p: 3, width: '100%' }}>
          <Tabs value={tab} onChange={(_, newValue) => setTab(newValue)}>
            <Tab label="Local Login" />
            <Tab label="Google Login" />
          </Tabs>
          {tab === 0 && (
            <Box component="form" onSubmit={handleSubmit(onSubmit)} sx={{ mt: 2 }}>
              {error && (
                <Alert severity="error" sx={{ mb: 2 }}>
                  {error}
                </Alert>
              )}
              <TextField
                margin="normal"
                required
                fullWidth
                id="username"
                label="Username"
                autoComplete="username"
                autoFocus
                {...register('username')}
                error={!!errors.username}
                helperText={errors.username?.message}
              />
              <TextField
                margin="normal"
                required
                fullWidth
                label="Password"
                type="password"
                id="password"
                autoComplete="current-password"
                {...register('password')}
                error={!!errors.password}
                helperText={errors.password?.message}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                sx={{ mt: 3, mb: 2 }}
                disabled={loading}
              >
                {loading ? 'Signing in...' : 'Sign In'}
              </Button>
            </Box>
          )}
          {tab === 1 && (
            <Box sx={{ mt: 2 }}>
              <Button
                fullWidth
                variant="outlined"
                onClick={handleGoogleLogin}
                sx={{ mt: 2 }}
              >
                Sign in with Google
              </Button>
            </Box>
          )}
        </Paper>
      </Box>
    </Container>
  )
}
