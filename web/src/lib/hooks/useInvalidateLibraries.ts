import { useQueryClient } from '@tanstack/react-query'

const useInvalidateLibraries = () => {
  const queryClient = useQueryClient()

  return () => {
    queryClient.invalidateQueries({ queryKey: ['/api/libraries'] })
  }
}

export { useInvalidateLibraries }
