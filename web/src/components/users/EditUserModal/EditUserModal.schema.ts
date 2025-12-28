import { z } from 'zod'

export const editUserSchema = z.object({
  displayName: z.string().max(100, 'Display name must be at most 100 characters'),
  isAdmin: z.boolean(),
})

export type EditUserValues = z.infer<typeof editUserSchema>
