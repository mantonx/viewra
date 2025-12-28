import { useState, useEffect } from 'react'
import { useAuth } from '@/contexts'
import { authFetch } from '@/lib/utils/authFetch'
import { Button, Modal, Loading } from '@/components/ui'
import { SettingsPage } from '@/components/common'
import { CreateUserForm, EditUserModal, ResetPasswordModal } from '@/components/users'
import { useToast } from '@/lib/hooks/useToast'
import { cn } from '@/lib/utils'
import { Plus, Trash2, Edit2, Shield, User as UserIcon } from 'lucide-react'
import type { GithubComMantonxViewraInternalApplicationAuthUserResponse as User } from '@/lib/api/generated/models'

export const UserManagement = () => {
  const { user: currentUser } = useAuth()
  const toast = useToast()

  // Users state
  const [users, setUsers] = useState<User[]>([])
  const [usersLoading, setUsersLoading] = useState(true)
  const [usersError, setUsersError] = useState<string | null>(null)

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [resetPasswordTarget, setResetPasswordTarget] = useState<{
    userId: string
    username: string
  } | null>(null)

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

  const handleCreateSuccess = (newUser: User) => {
    setUsers((prev) => [...prev, newUser])
    setShowCreateModal(false)
    toast.success('User created successfully')
  }

  const handleEditSuccess = (updatedUser: User) => {
    setUsers((prev) => prev.map((u) => (u.id === updatedUser.id ? updatedUser : u)))
    setEditingUser(null)
    toast.success('User updated successfully')
  }

  const handleDelete = async (userId: string) => {
    try {
      const response = await authFetch(`/api/users/${userId}`, {
        method: 'DELETE',
      })
      if (!response.ok) {
        throw new Error('Failed to delete user')
      }
      setUsers((prev) => prev.filter((u) => u.id !== userId))
      toast.success('User deleted successfully')
    } catch {
      toast.error('Failed to delete user')
    }
  }

  const handleResetPasswordSuccess = () => {
    setResetPasswordTarget(null)
    toast.success('Password reset successfully')
  }

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="User Management"
        description="Manage user accounts and permissions"
        actions={
          <Button onClick={() => setShowCreateModal(true)}>
            <Plus className="w-4 h-4 mr-1.5" />
            Add User
          </Button>
        }
      />

      <SettingsPage.Card title="Users">
        {usersLoading ? (
          <Loading text="Loading users..." />
        ) : usersError ? (
          <div className="py-4 text-center text-red-600 dark:text-red-400">{usersError}</div>
        ) : users.length === 0 ? (
          <div className="py-4 text-center text-neutral-500 dark:text-neutral-400">
            No users found
          </div>
        ) : (
          <SettingsPage.List>
            {users.map((user) => (
              <div
                key={user.id}
                className="flex items-center justify-between px-4 py-3 bg-white/50 dark:bg-white/[0.02] hover:bg-white dark:hover:bg-white/5 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <div
                    className={cn(
                      'w-10 h-10 rounded-full flex items-center justify-center',
                      user.is_admin
                        ? 'bg-amber-100 dark:bg-amber-900/30'
                        : 'bg-neutral-100 dark:bg-neutral-800'
                    )}
                  >
                    {user.is_admin ? (
                      <Shield className="w-5 h-5 text-amber-600 dark:text-amber-400" />
                    ) : (
                      <UserIcon className="w-5 h-5 text-neutral-500" />
                    )}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-neutral-900 dark:text-neutral-50">
                        {user.display_name || user.username}
                      </span>
                      {user.is_admin && (
                        <span className="text-xs px-1.5 py-0.5 rounded bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400">
                          Admin
                        </span>
                      )}
                    </div>
                    <span className="text-sm text-neutral-500 dark:text-neutral-400">
                      @{user.username}
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <Button variant="ghost" size="sm" onClick={() => setEditingUser(user)}>
                    <Edit2 className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      if (user.id && user.username) {
                        setResetPasswordTarget({ userId: user.id, username: user.username })
                      }
                    }}
                  >
                    Reset Password
                  </Button>
                  {user.id !== currentUser?.id && (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/30"
                      onClick={() => {
                        if (user.id) {
                          handleDelete(user.id)
                        }
                      }}
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </SettingsPage.List>
        )}
      </SettingsPage.Card>

      {/* Create User Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        title="Create User"
        size="sm"
      >
        <CreateUserForm
          onSuccess={handleCreateSuccess}
          onCancel={() => setShowCreateModal(false)}
        />
      </Modal>

      {/* Edit User Modal */}
      {editingUser && (
        <EditUserModal
          isOpen={true}
          user={editingUser}
          onSuccess={handleEditSuccess}
          onClose={() => setEditingUser(null)}
        />
      )}

      {/* Reset Password Modal */}
      <ResetPasswordModal
        isOpen={!!resetPasswordTarget}
        userId={resetPasswordTarget?.userId ?? ''}
        username={resetPasswordTarget?.username ?? ''}
        onSuccess={handleResetPasswordSuccess}
        onClose={() => setResetPasswordTarget(null)}
      />
    </SettingsPage>
  )
}
