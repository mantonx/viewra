export interface LoginFormProps {
  /** Called after successful login */
  onSuccess: () => void
  /** Whether to use glass variant inputs */
  variant?: 'default' | 'glass'
}
