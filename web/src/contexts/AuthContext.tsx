import { createContext, useContext, useCallback, useEffect, useState, type ReactNode } from 'react'

// Auth types
interface User {
  id: string
  username: string
  display_name: string
  is_admin: boolean
  created_at: string
}

interface AuthTokens {
  accessToken: string
  refreshToken: string
  expiresAt: number // Unix timestamp in ms
}

interface AuthState {
  user: User | null
  tokens: AuthTokens | null
  isAuthenticated: boolean
  isLoading: boolean
  needsSetup: boolean
}

interface AuthContextValue extends AuthState {
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<boolean>
  getAccessToken: () => Promise<string | null>
}

const AuthContext = createContext<AuthContextValue | null>(null)

// Storage keys
const STORAGE_KEY_TOKENS = 'viewra_auth_tokens'
const STORAGE_KEY_USER = 'viewra_auth_user'

// Token refresh buffer (refresh 1 minute before expiry)
const REFRESH_BUFFER_MS = 60 * 1000

// API base URL
const API_BASE = ''

// Helper to load stored auth data
const loadStoredAuth = (): { tokens: AuthTokens | null; user: User | null } => {
  try {
    const tokensStr = localStorage.getItem(STORAGE_KEY_TOKENS)
    const userStr = localStorage.getItem(STORAGE_KEY_USER)

    const tokens = tokensStr ? JSON.parse(tokensStr) : null
    const user = userStr ? JSON.parse(userStr) : null

    return { tokens, user }
  } catch {
    return { tokens: null, user: null }
  }
}

// Helper to save auth data
const saveAuth = (tokens: AuthTokens, user: User) => {
  localStorage.setItem(STORAGE_KEY_TOKENS, JSON.stringify(tokens))
  localStorage.setItem(STORAGE_KEY_USER, JSON.stringify(user))
}

// Helper to clear auth data
const clearAuth = () => {
  localStorage.removeItem(STORAGE_KEY_TOKENS)
  localStorage.removeItem(STORAGE_KEY_USER)
}

interface AuthProviderProps {
  children: ReactNode
}

const AuthProvider = ({ children }: AuthProviderProps) => {
  const [state, setState] = useState<AuthState>(() => {
    const { tokens, user } = loadStoredAuth()
    return {
      user,
      tokens,
      isAuthenticated: !!tokens && !!user,
      isLoading: true,
      needsSetup: false,
    }
  })

  // Listen for logout events from the API mutator (401 handling)
  useEffect(() => {
    const handleLogoutEvent = () => {
      setState(prev => ({
        ...prev,
        user: null,
        tokens: null,
        isAuthenticated: false,
      }))
    }

    window.addEventListener('auth:logout', handleLogoutEvent)
    return () => window.removeEventListener('auth:logout', handleLogoutEvent)
  }, [])

  // Check if setup is needed on mount, and auto-login in development mode
  useEffect(() => {
    const initAuth = async () => {
      try {
        const response = await fetch(`${API_BASE}/api/auth/setup`)
        const data = await response.json()
        const needsSetup = data.needs_setup === true

        // If we already have valid tokens, we're done
        const { tokens, user } = loadStoredAuth()
        if (tokens && user) {
          setState(prev => ({
            ...prev,
            needsSetup,
            isLoading: false,
          }))
          return
        }

        // Auto-login with dev credentials in development mode
        // This only works if the backend seeded a dev user
        if (import.meta.env.DEV && !needsSetup) {
          try {
            const loginResponse = await fetch(`${API_BASE}/api/auth/login`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ username: 'dev', password: 'devdev00' }),
            })

            if (loginResponse.ok) {
              const loginData = await loginResponse.json()
              const newTokens: AuthTokens = {
                accessToken: loginData.AccessToken,
                refreshToken: loginData.RefreshToken,
                expiresAt: Date.now() + loginData.ExpiresIn * 1000,
              }
              const newUser: User = {
                id: loginData.User.id,
                username: loginData.User.username,
                display_name: loginData.User.display_name,
                is_admin: loginData.User.is_admin,
                created_at: loginData.User.created_at,
              }

              saveAuth(newTokens, newUser)
              setState({
                user: newUser,
                tokens: newTokens,
                isAuthenticated: true,
                isLoading: false,
                needsSetup: false,
              })
              console.log('[Dev] Auto-logged in as dev user')
              return
            }
          } catch {
            // Auto-login failed, continue to normal flow
          }
        }

        setState(prev => ({
          ...prev,
          needsSetup,
          isLoading: false,
        }))
      } catch {
        setState(prev => ({ ...prev, isLoading: false }))
      }
    }

    initAuth()
  }, [])

  // Token refresh logic
  const refresh = useCallback(async (): Promise<boolean> => {
    const { tokens } = loadStoredAuth()
    if (!tokens?.refreshToken) {
      return false
    }

    try {
      const response = await fetch(`${API_BASE}/api/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: tokens.refreshToken }),
      })

      if (!response.ok) {
        clearAuth()
        setState(prev => ({
          ...prev,
          user: null,
          tokens: null,
          isAuthenticated: false,
        }))
        return false
      }

      const data = await response.json()
      const newTokens: AuthTokens = {
        accessToken: data.AccessToken,
        refreshToken: data.RefreshToken,
        expiresAt: Date.now() + data.ExpiresIn * 1000,
      }
      const user: User = {
        id: data.User.id,
        username: data.User.username,
        display_name: data.User.display_name,
        is_admin: data.User.is_admin,
        created_at: data.User.created_at,
      }

      saveAuth(newTokens, user)
      setState(prev => ({
        ...prev,
        user,
        tokens: newTokens,
        isAuthenticated: true,
      }))
      return true
    } catch {
      clearAuth()
      setState(prev => ({
        ...prev,
        user: null,
        tokens: null,
        isAuthenticated: false,
      }))
      return false
    }
  }, [])

  // Get a valid access token, refreshing if needed
  const getAccessToken = useCallback(async (): Promise<string | null> => {
    const { tokens } = loadStoredAuth()
    if (!tokens) {
      return null
    }

    // Check if token is about to expire
    if (tokens.expiresAt - Date.now() < REFRESH_BUFFER_MS) {
      const success = await refresh()
      if (!success) {
        return null
      }
      const refreshedTokens = loadStoredAuth().tokens
      return refreshedTokens?.accessToken ?? null
    }

    return tokens.accessToken
  }, [refresh])

  // Auto-refresh token before expiry
  useEffect(() => {
    if (!state.tokens) return

    const timeUntilRefresh = state.tokens.expiresAt - Date.now() - REFRESH_BUFFER_MS
    if (timeUntilRefresh <= 0) {
      refresh()
      return
    }

    const timer = setTimeout(() => {
      refresh()
    }, timeUntilRefresh)

    return () => clearTimeout(timer)
  }, [state.tokens, refresh])

  const login = useCallback(async (username: string, password: string) => {
    const response = await fetch(`${API_BASE}/api/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Login failed' }))
      throw new Error(error.error || 'Login failed')
    }

    const data = await response.json()
    const tokens: AuthTokens = {
      accessToken: data.AccessToken,
      refreshToken: data.RefreshToken,
      expiresAt: Date.now() + data.ExpiresIn * 1000,
    }
    const user: User = {
      id: data.User.id,
      username: data.User.username,
      display_name: data.User.display_name,
      is_admin: data.User.is_admin,
      created_at: data.User.created_at,
    }

    saveAuth(tokens, user)
    setState(prev => ({
      ...prev,
      user,
      tokens,
      isAuthenticated: true,
      needsSetup: false,
    }))
  }, [])

  const logout = useCallback(async () => {
    const { tokens } = loadStoredAuth()

    // Try to logout on server (fire-and-forget)
    if (tokens?.refreshToken) {
      fetch(`${API_BASE}/api/auth/logout`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${tokens.accessToken}`,
        },
        body: JSON.stringify({ refresh_token: tokens.refreshToken }),
      }).catch(() => {
        // Ignore errors - we're logging out anyway
      })
    }

    clearAuth()
    setState(prev => ({
      ...prev,
      user: null,
      tokens: null,
      isAuthenticated: false,
    }))
  }, [])

  const value: AuthContextValue = {
    ...state,
    login,
    logout,
    refresh,
    getAccessToken,
  }

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}

const useAuth = (): AuthContextValue => {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

export { AuthProvider, useAuth }
export type { User, AuthTokens, AuthState, AuthContextValue }
