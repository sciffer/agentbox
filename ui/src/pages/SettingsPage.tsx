import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  Alert,
} from '@mui/material'
import { authAPI } from '../services/api'
import { useAuthStore } from '../store/authStore'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const changePasswordSchema = z.object({
  currentPassword: z.string().min(1, 'Current password is required'),
  newPassword: z.string().min(8, 'New password must be at least 8 characters'),
  confirmPassword: z.string().min(1, 'Please confirm your password'),
}).refine((data) => data.newPassword === data.confirmPassword, {
  message: "Passwords don't match",
  path: ['confirmPassword'],
})

type ChangePasswordFormData = z.infer<typeof changePasswordSchema>

export default function SettingsPage() {
  const { user } = useAuthStore()
  const [success, setSuccess] = useState(false)

  const changePasswordMutation = useMutation({
    mutationFn: (data: { currentPassword: string; newPassword: string }) =>
      authAPI.changePassword(data.currentPassword, data.newPassword),
    onSuccess: () => {
      setSuccess(true)
      reset()
      setTimeout(() => setSuccess(false), 3000)
    },
  })

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<ChangePasswordFormData>({
    resolver: zodResolver(changePasswordSchema),
  })

  const onSubmit = (data: ChangePasswordFormData) => {
    changePasswordMutation.mutate({
      currentPassword: data.currentPassword,
      newPassword: data.newPassword,
    })
  }

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Settings
      </Typography>

      <Paper sx={{ p: 3, mt: 2 }}>
        <Typography variant="h6" gutterBottom>
          User Profile
        </Typography>
        <Typography><strong>Username:</strong> {user?.username}</Typography>
        <Typography><strong>Email:</strong> {user?.email || '-'}</Typography>
        <Typography><strong>Role:</strong> {user?.role}</Typography>
      </Paper>

      <Paper sx={{ p: 3, mt: 2 }}>
        <Typography variant="h6" gutterBottom>
          Change Password
        </Typography>
        {success && (
          <Alert severity="success" sx={{ mb: 2 }}>
            Password changed successfully!
          </Alert>
        )}
        <form onSubmit={handleSubmit(onSubmit)}>
          <TextField
            margin="normal"
            fullWidth
            label="Current Password"
            type="password"
            {...register('currentPassword')}
            error={!!errors.currentPassword}
            helperText={errors.currentPassword?.message}
          />
          <TextField
            margin="normal"
            fullWidth
            label="New Password"
            type="password"
            {...register('newPassword')}
            error={!!errors.newPassword}
            helperText={errors.newPassword?.message}
          />
          <TextField
            margin="normal"
            fullWidth
            label="Confirm New Password"
            type="password"
            {...register('confirmPassword')}
            error={!!errors.confirmPassword}
            helperText={errors.confirmPassword?.message}
          />
          <Button
            type="submit"
            variant="contained"
            sx={{ mt: 2 }}
            disabled={changePasswordMutation.isPending}
          >
            {changePasswordMutation.isPending ? 'Changing...' : 'Change Password'}
          </Button>
        </form>
      </Paper>
    </Box>
  )
}
