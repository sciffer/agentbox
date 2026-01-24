import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './store/authStore'
import Layout from './components/layout/Layout'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import EnvironmentsPage from './pages/EnvironmentsPage'
import EnvironmentDetailPage from './pages/EnvironmentDetailPage'
import UsersPage from './pages/UsersPage'
import APIKeysPage from './pages/APIKeysPage'
import SettingsPage from './pages/SettingsPage'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore()
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <PrivateRoute>
            <Layout />
          </PrivateRoute>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="environments" element={<EnvironmentsPage />} />
        <Route path="environments/:id" element={<EnvironmentDetailPage />} />
        <Route path="users" element={<UsersPage />} />
        <Route path="api-keys" element={<APIKeysPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  )
}

export default App
