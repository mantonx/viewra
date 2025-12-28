import { z } from 'zod'

export const createUserFormSchema = z.object({
  username: z
    .string()
    .min(1, 'Username is required')
    .min(3, 'Username must be at least 3 characters')
    .max(50, 'Username must be at most 50 characters')
    .regex(/^[a-zA-Z0-9_-]+$/, 'Username can only contain letters, numbers, underscores, and hyphens'),
  displayName: z.string().max(100, 'Display name must be at most 100 characters').optional(),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  isAdmin: z.boolean(),
})

export type CreateUserFormValues = z.infer<typeof createUserFormSchema>
