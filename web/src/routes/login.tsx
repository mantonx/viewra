import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect, type FormEvent } from 'react'
import { useAuth } from '@/contexts'
import { useTheme } from '@/contexts'
import { Button, Input, Alert, Loading } from '@/components/ui'
import { cn } from '@/lib/utils'

const LoginPage = () => {
  const navigate = useNavigate()
  const { login, isAuthenticated, isLoading: authLoading, needsSetup } = useAuth()
  const { theme } = useTheme()
  const isDark = theme === 'dark'

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

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

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsLoading(true)

    try {
      await login(username, password)
      navigate({ to: '/' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setIsLoading(false)
    }
  }

  if (authLoading) {
    return (
      <div className={cn(
        'flex items-center justify-center p-4',
        isDark ? 'auth-background' : 'auth-background-light'
      )}>
        <Loading size="lg" text="Loading..." />
      </div>
    )
  }

  return (
    <div className={cn(
      'flex items-center justify-center p-4 relative overflow-hidden',
      isDark ? 'auth-background' : 'auth-background-light'
    )}>
      {/* Ambient glow effect */}
      <div className="auth-glow -top-20 -left-20" />
      <div className="auth-glow -bottom-20 -right-20" style={{ animationDelay: '4s' }} />

      {/* Login card */}
      <div className={cn(
        'w-full max-w-md p-8 rounded-2xl relative z-10',
        isDark ? 'auth-card' : 'auth-card-light'
      )}>
        {/* Logo and heading */}
        <div className="text-center mb-8">
          <h1 className={cn(
            'text-4xl auth-logo',
            !isDark && 'auth-logo-light'
          )}>
            ViewRA
          </h1>
          <p className={cn(
            'mt-3 text-sm',
            isDark ? 'text-neutral-400' : 'text-neutral-600'
          )}>
            Sign in to your media server
          </p>
        </div>

        {/* Error alert */}
        {error && (
          <Alert variant="error" className="mb-6">
            {error}
          </Alert>
        )}

        {/* Form with staggered animation */}
        <form onSubmit={handleSubmit} className="auth-form-enter space-y-5">
          <Input
            label="Username"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoComplete="username"
            autoFocus
            variant="glass"
          />

          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
            variant="glass"
          />

          <Button
            type="submit"
            size="lg"
            className="w-full mt-4 rounded-lg font-semibold"
            isLoading={isLoading}
            disabled={!username || !password}
          >
            Sign In
          </Button>
        </form>
      </div>
    </div>
  )
}

export const Route = createFileRoute('/login')({
  component: LoginPage,
})
