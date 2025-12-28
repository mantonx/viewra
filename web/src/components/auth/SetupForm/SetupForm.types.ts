export interface SetupFormProps {
  /** Called after successful setup and login */
  onSuccess: () => void
  /** Whether to use glass variant inputs */
  variant?: 'default' | 'glass'
}
