import { z } from 'zod'

export const pathInputSchema = z.object({
  path: z
    .string()
    .min(1, 'Please enter a path')
    .refine((val) => val.startsWith('/'), 'Path must start with /'),
})

export type PathInputValues = z.infer<typeof pathInputSchema>
