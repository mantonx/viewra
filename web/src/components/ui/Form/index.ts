/**
 * Form Adapter Components for TanStack Form
 *
 * These components connect TanStack Form's field API to ViewRA's UI components,
 * providing automatic value binding, change handling, and error display.
 *
 * @example
 * import { useForm } from '@tanstack/react-form'
 * import { z } from 'zod'
 * import { FormInput, FormPasswordInput, FormToggle } from '@/components/ui/Form'
 *
 * const schema = z.object({
 *   username: z.string().min(1, 'Required'),
 *   password: z.string().min(8, 'At least 8 characters'),
 *   rememberMe: z.boolean(),
 * })
 *
 * const form = useForm({
 *   defaultValues: { username: '', password: '', rememberMe: false },
 *   validators: { onChange: schema },
 *   onSubmit: async ({ value }) => { ... },
 * })
 */

// Field adapters
export { FormInput } from './FormInput'
export { FormNumberInput } from './FormNumberInput'
export { FormPasswordInput } from './FormPasswordInput'
export { FormToggle } from './FormToggle'
export { FormSelect } from './FormSelect'
export { FormSettingToggle } from './FormSettingToggle'

// Form-level components
export { FormSubmitButton } from './FormSubmitButton'
export { FormModalFooter } from './FormModalFooter'
export { FormApiError } from './FormApiError'
export { FormSectionTitle } from './FormSectionTitle'

// Types and utilities
export { getFieldError, type FieldComponentProps, type AnyFormApi } from './Form.types'
