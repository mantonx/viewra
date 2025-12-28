import { useEffect, useState, useCallback, useRef } from 'react'
import { getErrorMessage } from '@/lib/utils/error'

type UseFormResetOptions<TDeps extends unknown[]> = {
  /** Whether the form should be active (e.g., modal is open) */
  isOpen: boolean
  /** Dependencies that should trigger a form reset (besides isOpen) */
  deps?: TDeps
}

type UseFormResetReturn = {
  apiError: string | null
  setApiError: (error: string | null) => void
  clearApiError: () => void
  handleSubmitError: (error: unknown) => void
  /** Key that changes when form should reset - use with useForm's key or useEffect */
  resetKey: number
}

/**
 * Hook that manages form reset state and API error handling for modal forms.
 *
 * Use this alongside TanStack Form's useForm to get:
 * - A resetKey that changes when the form should be reset
 * - API error state management
 * - Consistent error handling
 *
 * @example
 * const { apiError, handleSubmitError, resetKey } = useFormReset({
 *   isOpen,
 *   deps: [library],
 * })
 *
 * const form = useForm({
 *   defaultValues: getDefaultValues(library),
 *   validators: { onChange: schema },
 *   onSubmit: async ({ value }) => {
 *     try {
 *       await mutation.mutateAsync({ data: value })
 *       onClose()
 *     } catch (error) {
 *       handleSubmitError(error)
 *     }
 *   },
 * })
 *
 * // Reset form when resetKey changes
 * useEffect(() => {
 *   if (isOpen) {
 *     form.reset(getDefaultValues(library))
 *   }
 * }, [resetKey])
 */
export const useFormReset = <TDeps extends unknown[] = []>({
  isOpen,
  deps = [] as unknown as TDeps,
}: UseFormResetOptions<TDeps>): UseFormResetReturn => {
  const [apiError, setApiError] = useState<string | null>(null)
  const [resetKey, setResetKey] = useState(0)
  const prevIsOpenRef = useRef(isOpen)

  const clearApiError = useCallback(() => setApiError(null), [])

  const handleSubmitError = useCallback((error: unknown) => {
    setApiError(getErrorMessage(error))
  }, [])

  // Increment resetKey when modal opens or deps change while open
  useEffect(() => {
    const wasOpen = prevIsOpenRef.current
    prevIsOpenRef.current = isOpen

    if (isOpen) {
      // Either just opened, or deps changed while open
      if (!wasOpen || deps.length > 0) {
        setResetKey((k) => k + 1)
        setApiError(null)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, ...deps])

  return {
    apiError,
    setApiError,
    clearApiError,
    handleSubmitError,
    resetKey,
  }
}
