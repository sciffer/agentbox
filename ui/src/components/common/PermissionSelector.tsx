import { useState, useEffect } from 'react'
import { Environment, APIKeyPermission } from '../../types'
import { environmentsAPI } from '../../services/api'

interface PermissionSelectorProps {
  selectedPermissions: APIKeyPermission[]
  onChange: (permissions: APIKeyPermission[]) => void
  maxLevel?: 'viewer' | 'editor' | 'owner'
  disabled?: boolean
}

const permissionLevels = ['viewer', 'editor', 'owner'] as const
type PermissionLevel = typeof permissionLevels[number]

const permissionColors: Record<PermissionLevel, string> = {
  viewer: 'bg-blue-100 text-blue-800',
  editor: 'bg-yellow-100 text-yellow-800',
  owner: 'bg-green-100 text-green-800',
}

export function PermissionSelector({
  selectedPermissions,
  onChange,
  maxLevel = 'owner',
  disabled = false,
}: PermissionSelectorProps) {
  const [environments, setEnvironments] = useState<Environment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        setLoading(true)
        const data = await environmentsAPI.list({ limit: 100 })
        setEnvironments(data.environments || [])
        setError(null)
      } catch (err) {
        setError('Failed to load environments')
        console.error('Failed to fetch environments:', err)
      } finally {
        setLoading(false)
      }
    }
    fetchEnvironments()
  }, [])

  const getMaxLevelIndex = () => permissionLevels.indexOf(maxLevel)

  const handleToggleEnvironment = (envId: string) => {
    const existing = selectedPermissions.find((p) => p.environment_id === envId)
    if (existing) {
      onChange(selectedPermissions.filter((p) => p.environment_id !== envId))
    } else {
      onChange([
        ...selectedPermissions,
        { environment_id: envId, permission: 'viewer' },
      ])
    }
  }

  const handlePermissionChange = (envId: string, permission: PermissionLevel) => {
    onChange(
      selectedPermissions.map((p) =>
        p.environment_id === envId ? { ...p, permission } : p
      )
    )
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-4">
        <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-blue-600"></div>
        <span className="ml-2 text-sm text-gray-500">Loading environments...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-red-600 text-sm py-2">{error}</div>
    )
  }

  if (environments.length === 0) {
    return (
      <div className="text-gray-500 text-sm py-2">
        No environments available. Create an environment first.
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <label className="block text-sm font-medium text-gray-700">
        Environment Permissions
      </label>
      <div className="border rounded-md divide-y max-h-60 overflow-y-auto">
        {environments.map((env) => {
          const selected = selectedPermissions.find(
            (p) => p.environment_id === env.id
          )
          return (
            <div
              key={env.id}
              className={`flex items-center justify-between p-3 ${
                disabled ? 'opacity-50' : ''
              }`}
            >
              <div className="flex items-center">
                <input
                  type="checkbox"
                  id={`env-${env.id}`}
                  checked={!!selected}
                  onChange={() => handleToggleEnvironment(env.id)}
                  disabled={disabled}
                  className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                />
                <label
                  htmlFor={`env-${env.id}`}
                  className="ml-3 text-sm text-gray-700"
                >
                  {env.name}
                  <span
                    className={`ml-2 inline-flex px-2 py-0.5 text-xs rounded-full ${
                      env.status === 'running'
                        ? 'bg-green-100 text-green-800'
                        : env.status === 'pending'
                        ? 'bg-yellow-100 text-yellow-800'
                        : 'bg-gray-100 text-gray-800'
                    }`}
                  >
                    {env.status}
                  </span>
                </label>
              </div>
              {selected && (
                <select
                  value={selected.permission}
                  onChange={(e) =>
                    handlePermissionChange(env.id, e.target.value as PermissionLevel)
                  }
                  disabled={disabled}
                  className={`text-sm border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 ${
                    permissionColors[selected.permission]
                  }`}
                >
                  {permissionLevels.slice(0, getMaxLevelIndex() + 1).map((level) => (
                    <option key={level} value={level}>
                      {level.charAt(0).toUpperCase() + level.slice(1)}
                    </option>
                  ))}
                </select>
              )}
            </div>
          )
        })}
      </div>
      {selectedPermissions.length > 0 && (
        <div className="text-xs text-gray-500 mt-1">
          {selectedPermissions.length} environment(s) selected
        </div>
      )}
    </div>
  )
}

// Compact display of permissions (for tables)
interface PermissionChipsProps {
  permissions: APIKeyPermission[]
}

export function PermissionChips({ permissions }: PermissionChipsProps) {
  if (!permissions || permissions.length === 0) {
    return <span className="text-gray-400 text-sm">No restrictions</span>
  }

  const displayCount = 3
  const showMore = permissions.length > displayCount

  return (
    <div className="flex flex-wrap gap-1">
      {permissions.slice(0, displayCount).map((p) => (
        <span
          key={p.environment_id}
          className={`inline-flex px-2 py-0.5 text-xs rounded-full ${
            permissionColors[p.permission]
          }`}
          title={`${p.environment_id}: ${p.permission}`}
        >
          {p.environment_id.slice(0, 8)}...:{p.permission.charAt(0).toUpperCase()}
        </span>
      ))}
      {showMore && (
        <span className="inline-flex px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-600">
          +{permissions.length - displayCount} more
        </span>
      )}
    </div>
  )
}
