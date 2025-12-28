import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useAuth } from '@/contexts'
import { useTheme } from '@/contexts'
import { Loading } from '@/components/ui'
import { LoginForm } from '@/components/auth/LoginForm'
import { cn } from '@/lib/utils'

const LoginPage = () => {
  const navigate = useNavigate()
  const { isAuthenticated, isLoading: authLoading, needsSetup } = useAuth()
  const { theme } = useTheme()
  const isDark = theme === 'dark'

  // Redirect if already authenticated
  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      navigate({ to: '/' })
    }
  }, [isAuthenticated, authLoading, navigate])

  // Redirect to setup if needed
  useEffect(() => {
    if (!authLoading && needsSetup) {
      navigate({ to: '/setup' })
    }
  }, [needsSetup, authLoading, navigate])

  const handleSuccess = () => {
    navigate({ to: '/' })
  }

  if (authLoading) {
    return (
      <div
        className={cn(
          'flex items-center justify-center p-4',
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
        'flex items-center justify-center p-4 relative overflow-hidden',
        isDark ? 'auth-background' : 'auth-background-light'
      )}
    >
      {/* Ambient glow effect */}
      <div className="auth-glow -top-20 -left-20" />
      <div className="auth-glow -bottom-20 -right-20" style={{ animationDelay: '4s' }} />

      {/* Login card */}
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
            Sign in to your media server
          </p>
        </div>

        <LoginForm onSuccess={handleSuccess} variant="glass" />
      </div>
    </div>
  )
}

export const Route = createFileRoute('/login')({
  component: LoginPage,
})
