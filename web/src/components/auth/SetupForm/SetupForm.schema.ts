import { z } from 'zod'

export const setupFormSchema = z
  .object({
    username: z.string().min(1, 'Username is required'),
    displayName: z.string().optional(),
    password: z.string().min(8, 'Password must be at least 8 characters'),
    confirmPassword: z.string().min(1, 'Please confirm your password'),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'Passwords do not match',
    path: ['confirmPassword'],
  })

export type SetupFormValues = z.infer<typeof setupFormSchema>
