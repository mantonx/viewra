import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect, type FormEvent } from 'react'
import { useAuth } from '@/contexts'
import { useTheme } from '@/contexts'
import { Button, Input, Alert, Loading } from '@/components/ui'
import { cn } from '@/lib/utils'

const SetupPage = () => {
  const navigate = useNavigate()
  const { isAuthenticated, isLoading: authLoading, needsSetup, login } = useAuth()
  const { theme } = useTheme()
  const isDark = theme === 'dark'

  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

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

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)

    // Validation
    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }

    setIsLoading(true)

    try {
      // Create admin account
      const response = await fetch('/api/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username,
          password,
          display_name: displayName || username,
        }),
      })

      if (!response.ok) {
        const data = await response.json().catch(() => ({ error: 'Setup failed' }))
        throw new Error(data.error || 'Setup failed')
      }

      // Auto-login after setup
      await login(username, password)
      navigate({ to: '/' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Setup failed')
    } finally {
      setIsLoading(false)
    }
  }

  if (authLoading) {
    return (
      <div className={cn(
        'min-h-screen flex items-center justify-center p-4',
        isDark ? 'auth-background' : 'auth-background-light'
      )}>
        <Loading size="lg" text="Loading..." />
      </div>
    )
  }

  return (
    <div className={cn(
      'min-h-screen flex items-center justify-center p-4 relative overflow-hidden',
      isDark ? 'auth-background' : 'auth-background-light'
    )}>
      {/* Ambient glow effect */}
      <div className="auth-glow -top-20 -left-20" />
      <div className="auth-glow -bottom-20 -right-20" style={{ animationDelay: '4s' }} />

      {/* Setup card */}
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
            Create your administrator account
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
            helperText="This will be used to sign in"
          />

          <Input
            label="Display Name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder={username || 'Optional'}
            autoComplete="name"
            variant="glass"
          />

          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="new-password"
            variant="glass"
            helperText="At least 8 characters"
          />

          <Input
            label="Confirm Password"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            autoComplete="new-password"
            variant="glass"
            error={confirmPassword && password !== confirmPassword ? 'Passwords do not match' : undefined}
          />

          <Button
            type="submit"
            size="lg"
            className="w-full mt-4 rounded-lg font-semibold"
            isLoading={isLoading}
            disabled={!username || !password || !confirmPassword}
          >
            Create Account
          </Button>
        </form>
      </div>
    </div>
  )
}

export const Route = createFileRoute('/setup')({
  component: SetupPage,
})
