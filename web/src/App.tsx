import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { routeTree } from './routeTree.gen'
import { ToastProvider, useToastState } from '@/lib/hooks/useToast'
import { ConfirmProvider, useConfirmState } from '@/lib/hooks/useConfirm'
import { ToastContainer, ConfirmDialog } from '@/components/ui'
import { ThemeProvider, AuthProvider, ServerStatusProvider } from '@/contexts'
import { ServerStatusOverlay } from '@/components/common'

// Create a new query client instance
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // 5 minutes
      retry: 1,
    },
  },
})

// Create a new router instance
const router = createRouter({
  routeTree,
  context: {
    queryClient,
  },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
})

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const AppProviders = ({ children }: { children: React.ReactNode }) => {
  const toastState = useToastState()
  const confirmState = useConfirmState()

  return (
    <ToastProvider value={toastState}>
      <ConfirmProvider value={confirmState}>
        {children}
        <ToastContainer toasts={toastState.toasts} onDismiss={toastState.removeToast} />
        <ConfirmDialog
          isOpen={confirmState.confirmState.isOpen}
          title={confirmState.confirmState.title}
          message={confirmState.confirmState.message}
          confirmText={confirmState.confirmState.confirmText}
          cancelText={confirmState.confirmState.cancelText}
          variant={confirmState.confirmState.variant}
          onConfirm={confirmState.handleConfirm}
          onCancel={confirmState.handleCancel}
        />
      </ConfirmProvider>
    </ToastProvider>
  )
}

const App = () => {
  return (
    <ThemeProvider>
      <AuthProvider>
        <QueryClientProvider client={queryClient}>
          <ServerStatusProvider>
            <AppProviders>
              <RouterProvider router={router} />
              <ServerStatusOverlay />
            </AppProviders>
          </ServerStatusProvider>
        </QueryClientProvider>
      </AuthProvider>
    </ThemeProvider>
  )
}

export { App }
