import { useState, useCallback, createContext, useContext } from 'react'
import type { ConfirmOptions } from '@/components/ui/ConfirmDialog'

interface ConfirmState extends ConfirmOptions {
  isOpen: boolean
  resolver?: (value: boolean) => void
}

interface ConfirmContextValue {
  confirmState: ConfirmState
  confirm: (options: ConfirmOptions) => Promise<boolean>
  handleConfirm: () => void
  handleCancel: () => void
}

const ConfirmContext = createContext<ConfirmContextValue | undefined>(undefined)

export const useConfirmState = () => {
  const [confirmState, setConfirmState] = useState<ConfirmState>({
    isOpen: false,
    title: '',
    message: '',
    confirmText: 'Confirm',
    cancelText: 'Cancel',
    variant: 'primary',
  })

  const confirm = useCallback((options: ConfirmOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      setConfirmState({
        isOpen: true,
        ...options,
        resolver: resolve,
      })
    })
  }, [])

  const handleConfirm = useCallback(() => {
    if (confirmState.resolver) {
      confirmState.resolver(true)
    }
    setConfirmState((prev) => ({ ...prev, isOpen: false, resolver: undefined }))
  }, [confirmState])

  const handleCancel = useCallback(() => {
    if (confirmState.resolver) {
      confirmState.resolver(false)
    }
    setConfirmState((prev) => ({ ...prev, isOpen: false, resolver: undefined }))
  }, [confirmState])

  return { confirmState, confirm, handleConfirm, handleCancel }
}

export const ConfirmProvider = ConfirmContext.Provider

export const useConfirm = () => {
  const context = useContext(ConfirmContext)
  if (!context) {
    throw new Error('useConfirm must be used within a ConfirmProvider')
  }
  return { confirm: context.confirm }
}

export const useConfirmDialog = () => {
  const context = useContext(ConfirmContext)
  if (!context) {
    throw new Error('useConfirmDialog must be used within a ConfirmProvider')
  }
  return {
    confirmState: context.confirmState,
    handleConfirm: context.handleConfirm,
    handleCancel: context.handleCancel,
  }
}
