import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useAuth } from '@/contexts'
import { Loading } from '@/components/ui'

type AdminRouteProps = {
  children: React.ReactNode
  /** Where to redirect non-admins. Defaults to /settings/account */
  redirectTo?: string
}

/**
 * Wrapper component that redirects non-admin users.
 * Use this to wrap admin-only page components in route files.
 *
 * @example
 * ```tsx
 * export const Route = createFileRoute('/_layout/settings/system')({
 *   component: () => (
 *     <AdminRoute>
 *       <SystemSettings />
 *     </AdminRoute>
 *   ),
 * })
 * ```
 */
export const AdminRoute = ({ children, redirectTo = '/settings/account' }: AdminRouteProps) => {
  const navigate = useNavigate()
  const { user, isLoading } = useAuth()

  useEffect(() => {
    if (!isLoading && user && !user.is_admin) {
      navigate({ to: redirectTo })
    }
  }, [user, isLoading, navigate, redirectTo])

  // Show loading while checking auth
  if (isLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Loading text="Checking permissions..." />
      </div>
    )
  }

  // Don't render if not admin (will redirect)
  if (!user?.is_admin) {
    return null
  }

  return <>{children}</>
}
