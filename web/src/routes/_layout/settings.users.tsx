import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect, type FormEvent } from 'react'
import { useAuth } from '@/contexts'
import { authFetch } from '@/lib/utils/authFetch'
import { Card, CardHeader, CardContent, Button, Input, Alert, Modal, ModalContent, ModalFooter, Toggle, Loading } from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Plus, Trash2, Edit2, X, Check, Shield, User as UserIcon } from 'lucide-react'
import type { GithubComMantonxViewraInternalApplicationAuthUserResponse as User } from '@/lib/api/generated/models'

const UserManagement = () => {
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()
  const toast = useToast()

  // Redirect non-admins
  useEffect(() => {
    if (currentUser && !currentUser.is_admin) {
      navigate({ to: '/settings/account' })
    }
  }, [currentUser, navigate])

  // Users state
  const [users, setUsers] = useState<User[]>([])
  const [usersLoading, setUsersLoading] = useState(true)
  const [usersError, setUsersError] = useState<string | null>(null)

  // Create user modal state
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [createUsername, setCreateUsername] = useState('')
  const [createDisplayName, setCreateDisplayName] = useState('')
  const [createPassword, setCreatePassword] = useState('')
  const [createIsAdmin, setCreateIsAdmin] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  // Edit user state
  const [editingUser, setEditingUser] = useState<string | null>(null)
  const [editDisplayName, setEditDisplayName] = useState('')
  const [editIsAdmin, setEditIsAdmin] = useState(false)

  // Fetch users on mount
  useEffect(() => {
    const fetchUsers = async () => {
      try {
        const response = await authFetch('/api/users')
        if (!response.ok) {
          throw new Error('Failed to fetch users')
        }
        const data = await response.json()
        setUsers(data.users || [])
      } catch (err) {
        setUsersError(err instanceof Error ? err.message : 'Failed to load users')
      } finally {
        setUsersLoading(false)
      }
    }

    fetchUsers()
  }, [])

  const handleCreateUser = async (e: FormEvent) => {
    e.preventDefault()
    setCreateError(null)

    if (createPassword.length < 8) {
      setCreateError('Password must be at least 8 characters')
      return
    }

    setCreateLoading(true)

    try {
      const response = await authFetch('/api/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: createUsername,
          display_name: createDisplayName || createUsername,
          password: createPassword,
          is_admin: createIsAdmin,
        }),
      })

      if (!response.ok) {
        const data = await response.json().catch(() => ({}))
        throw new Error(data.error || 'Failed to create user')
      }

      const newUser = await response.json()
      setUsers(prev => [...prev, newUser])
      toast.success('User created successfully')

      // Reset form
      setShowCreateModal(false)
      setCreateUsername('')
      setCreateDisplayName('')
      setCreatePassword('')
      setCreateIsAdmin(false)
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Failed to create user')
    } finally {
      setCreateLoading(false)
    }
  }

  const handleEditUser = async (userId: string) => {
    try {
      const response = await authFetch(`/api/users/${userId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          display_name: editDisplayName,
          is_admin: editIsAdmin,
        }),
      })

      if (!response.ok) {
        throw new Error('Failed to update user')
      }

      const updatedUser = await response.json()
      setUsers(prev => prev.map(u => u.id === userId ? updatedUser : u))
      setEditingUser(null)
      toast.success('User updated')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update user')
    }
  }

  const handleDeleteUser = async (userId: string, username: string) => {
    if (!confirm(`Are you sure you want to delete user "${username}"? This cannot be undone.`)) {
      return
    }

    try {
      const response = await authFetch(`/api/users/${userId}`, {
        method: 'DELETE',
      })

      if (!response.ok) {
        throw new Error('Failed to delete user')
      }

      setUsers(prev => prev.filter(u => u.id !== userId))
      toast.success('User deleted')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete user')
    }
  }

  const handleResetPassword = async (userId: string, username: string) => {
    const newPassword = prompt(`Enter new password for "${username}" (min 8 characters):`)
    if (!newPassword) {
      return
    }

    if (newPassword.length < 8) {
      toast.error('Password must be at least 8 characters')
      return
    }

    try {
      const response = await authFetch(`/api/users/${userId}/reset-password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ new_password: newPassword }),
      })

      if (!response.ok) {
        throw new Error('Failed to reset password')
      }

      toast.success(`Password reset for ${username}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to reset password')
    }
  }

  const startEditing = (user: User) => {
    if (!user.id) { return }
    setEditingUser(user.id)
    setEditDisplayName(user.display_name || '')
    setEditIsAdmin(user.is_admin || false)
  }

  const cancelEditing = () => {
    setEditingUser(null)
    setEditDisplayName('')
    setEditIsAdmin(false)
  }

  if (!currentUser?.is_admin) {
    return null
  }

  return (
    <div className="p-8 page-enter">
      <PageHeader
        title="User Management"
        description="Create and manage user accounts"
        actions={
          <Button onClick={() => setShowCreateModal(true)}>
            <Plus className="w-4 h-4 mr-2" />
            Add User
          </Button>
        }
      />

      {/* Users List */}
      <Card className="mt-6">
        <CardHeader>
          <h2 className={cn('text-lg font-semibold', text.primary)}>
            Users ({users.length})
          </h2>
        </CardHeader>
        <CardContent>
          {usersLoading ? (
            <Loading text="Loading users..." />
          ) : usersError ? (
            <Alert variant="error">{usersError}</Alert>
          ) : users.length === 0 ? (
            <div className={cn('text-sm', text.secondary)}>No users found</div>
          ) : (
            <div className="space-y-2">
              {users.map((user) => (
                <div
                  key={user.id}
                  className={cn(
                    'flex items-center justify-between p-4 rounded-lg',
                    'bg-neutral-50 dark:bg-neutral-800/50'
                  )}
                >
                  <div className="flex items-center gap-3 flex-1">
                    <div className={cn(
                      'p-2 rounded-full',
                      user.is_admin
                        ? 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400'
                        : 'bg-neutral-200 text-neutral-600 dark:bg-neutral-700 dark:text-neutral-400'
                    )}>
                      {user.is_admin ? <Shield className="w-5 h-5" /> : <UserIcon className="w-5 h-5" />}
                    </div>

                    {editingUser === user.id ? (
                      <div className="flex items-center gap-3 flex-1">
                        <Input
                          value={editDisplayName}
                          onChange={(e) => setEditDisplayName(e.target.value)}
                          placeholder="Display name"
                          className="max-w-48"
                        />
                        <div className="flex items-center gap-2">
                          <Toggle
                            enabled={editIsAdmin}
                            onChange={setEditIsAdmin}
                            label="Admin"
                            disabled={user.id === currentUser?.id}
                          />
                          <span className={cn('text-sm', text.secondary)}>Admin</span>
                        </div>
                      </div>
                    ) : (
                      <div>
                        <div className={cn('font-medium', text.primary)}>
                          {user.display_name || user.username}
                          {user.id === currentUser?.id && (
                            <span className="ml-2 text-xs text-blue-600 dark:text-blue-400">
                              (You)
                            </span>
                          )}
                        </div>
                        <div className={cn('text-sm', text.secondary)}>
                          @{user.username}
                          {user.is_admin && (
                            <span className="ml-2 text-amber-600 dark:text-amber-400">
                              • Administrator
                            </span>
                          )}
                        </div>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center gap-2">
                    {editingUser === user.id ? (
                      <>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => user.id && handleEditUser(user.id)}
                          className="text-green-600 hover:text-green-700 hover:bg-green-50 dark:text-green-400 dark:hover:bg-green-900/20"
                        >
                          <Check className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={cancelEditing}
                          className="text-neutral-600 hover:text-neutral-700 dark:text-neutral-400"
                        >
                          <X className="w-4 h-4" />
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => startEditing(user)}
                          title="Edit user"
                        >
                          <Edit2 className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => user.id && user.username && handleResetPassword(user.id, user.username)}
                          title="Reset password"
                          className="text-amber-600 hover:text-amber-700 hover:bg-amber-50 dark:text-amber-400 dark:hover:bg-amber-900/20"
                        >
                          Reset PW
                        </Button>
                        {user.id !== currentUser?.id && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => user.id && user.username && handleDeleteUser(user.id, user.username)}
                            className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                            title="Delete user"
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        )}
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Create User Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        title="Create User"
        size="sm"
      >
        <form onSubmit={handleCreateUser}>
          <ModalContent>
            {createError && (
              <Alert variant="error" className="mb-4">
                {createError}
              </Alert>
            )}

            <div className="space-y-4">
              <Input
                label="Username"
                type="text"
                value={createUsername}
                onChange={(e) => setCreateUsername(e.target.value)}
                required
                autoComplete="off"
                helperText="Used for signing in"
              />

              <Input
                label="Display Name"
                type="text"
                value={createDisplayName}
                onChange={(e) => setCreateDisplayName(e.target.value)}
                placeholder={createUsername || 'Optional'}
                autoComplete="off"
              />

              <Input
                label="Password"
                type="password"
                value={createPassword}
                onChange={(e) => setCreatePassword(e.target.value)}
                required
                autoComplete="new-password"
                helperText="At least 8 characters"
              />

              <div className="flex items-center gap-3">
                <Toggle
                  enabled={createIsAdmin}
                  onChange={setCreateIsAdmin}
                  label="Administrator"
                />
                <div>
                  <span className={text.primary}>Administrator</span>
                  <p className={cn('text-xs', text.secondary)}>
                    Can manage users and system settings
                  </p>
                </div>
              </div>
            </div>
          </ModalContent>

          <ModalFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setShowCreateModal(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              isLoading={createLoading}
              disabled={!createUsername || !createPassword}
            >
              Create User
            </Button>
          </ModalFooter>
        </form>
      </Modal>
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/users')({
  component: UserManagement,
})
