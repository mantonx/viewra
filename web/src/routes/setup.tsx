import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useAuth } from '@/contexts'
import { useTheme } from '@/contexts'
import { Loading } from '@/components/ui'
import { SetupForm } from '@/components/auth/SetupForm'
import { cn } from '@/lib/utils'

const SetupPage = () => {
  const navigate = useNavigate()
  const { isAuthenticated, isLoading: authLoading, needsSetup } = useAuth()
  const { theme } = useTheme()
  const isDark = theme === 'dark'

  // Redirect if already authenticated or setup not needed
  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      navigate({ to: '/' })
    }
  }, [isAuthenticated, authLoading, navigate])

  useEffect(() => {
    if (!authLoading && !needsSetup && !isAuthenticated) {
      navigate({ to: '/login' })
    }
  }, [needsSetup, authLoading, isAuthenticated, navigate])

  const handleSuccess = () => {
    navigate({ to: '/' })
  }

  if (authLoading) {
    return (
      <div
        className={cn(
          'min-h-screen flex items-center justify-center p-4',
          isDark ? 'auth-background' : 'auth-background-light'
        )}
      >
        <Loading size="lg" text="Loading..." />
      </div>
    )
  }

  return (
    <div
      className={cn(
        'min-h-screen flex items-center justify-center p-4 relative overflow-hidden',
        isDark ? 'auth-background' : 'auth-background-light'
      )}
    >
      {/* Ambient glow effect */}
      <div className="auth-glow -top-20 -left-20" />
      <div className="auth-glow -bottom-20 -right-20" style={{ animationDelay: '4s' }} />

      {/* Setup card */}
      <div
        className={cn(
          'w-full max-w-md p-8 rounded-2xl relative z-10',
          isDark ? 'auth-card' : 'auth-card-light'
        )}
      >
        {/* Logo and heading */}
        <div className="text-center mb-8">
          <h1 className={cn('text-4xl auth-logo', !isDark && 'auth-logo-light')}>ViewRA</h1>
          <p className={cn('mt-3 text-sm', isDark ? 'text-neutral-400' : 'text-neutral-600')}>
            Create your administrator account
          </p>
        </div>

        <SetupForm onSuccess={handleSuccess} variant="glass" />
      </div>
    </div>
  )
}

export const Route = createFileRoute('/setup')({
  component: SetupPage,
})
