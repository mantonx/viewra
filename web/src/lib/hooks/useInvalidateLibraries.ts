import { useQueryClient } from '@tanstack/react-query'

const useInvalidateLibraries = () => {
  const queryClient = useQueryClient()

  return () => {
    queryClient.invalidateQueries({ queryKey: ['LibrariesService', 'getApiLibraries'] })
  }
}

export { useInvalidateLibraries }
