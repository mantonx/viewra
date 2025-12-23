import { useState, useEffect, useCallback } from 'react'
import {
  useGetApiSettingsLocation,
  usePutApiSettingsLocation,
} from '@/lib/api/generated/settings/settings'
import { useToast } from './useToast'

export interface LocationResult {
  name: string
  country: string
  admin1?: string
  latitude: number
  longitude: number
  timezone: string
}

export const useLocationSettings = () => {
  const toast = useToast()
  const { data: locationData } = useGetApiSettingsLocation()
  const updateMutation = usePutApiSettingsLocation()

  const [enabled, setEnabled] = useState(false)
  const [latitude, setLatitude] = useState('')
  const [longitude, setLongitude] = useState('')
  const [timezone, setTimezone] = useState('auto')
  const [locationName, setLocationName] = useState('')
  const [isChanging, setIsChanging] = useState(false)

  // Search state
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<LocationResult[]>([])
  const [isSearching, setIsSearching] = useState(false)

  // Sync with server data
  useEffect(() => {
    if (locationData?.status !== 200) {return}
    if (isChanging) {return} // Don't overwrite while user is changing

    const data = locationData.data as {
      enabled?: boolean
      latitude?: number
      longitude?: number
      timezone?: string
      location_name?: string
    }

    setEnabled(data.enabled ?? false)
    setLatitude(data.latitude?.toString() ?? '')
    setLongitude(data.longitude?.toString() ?? '')
    setTimezone(data.timezone ?? 'auto')
    setLocationName(data.location_name ?? '')
  }, [locationData, isChanging])

  const search = useCallback(async (query: string) => {
    if (!query.trim()) {return}

    setIsSearching(true)
    setSearchResults([])

    try {
      const res = await fetch(
        `https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(query)}&count=5&language=en&format=json`
      )
      const data = await res.json()

      if (data.results?.length > 0) {
        setSearchResults(
          data.results.map((r: LocationResult) => ({
            name: r.name,
            country: r.country,
            admin1: r.admin1,
            latitude: r.latitude,
            longitude: r.longitude,
            timezone: r.timezone,
          }))
        )
      } else {
        toast.error('No locations found')
      }
    } catch {
      toast.error('Failed to search locations')
    } finally {
      setIsSearching(false)
    }
  }, [toast])

  const selectLocation = useCallback(async (result: LocationResult) => {
    const name = result.admin1
      ? `${result.name}, ${result.admin1}, ${result.country}`
      : `${result.name}, ${result.country}`

    setLatitude(result.latitude.toFixed(4))
    setLongitude(result.longitude.toFixed(4))
    setTimezone(result.timezone)
    setLocationName(name)
    setEnabled(true)
    setIsChanging(false)
    setSearchResults([])
    setSearchQuery('')

    try {
      await updateMutation.mutateAsync({
        data: {
          enabled: true,
          latitude: result.latitude,
          longitude: result.longitude,
          timezone: result.timezone,
          location_name: name,
        },
      })
      toast.success(`Location set to ${result.name}`)
    } catch {
      toast.error('Failed to save location')
    }
  }, [updateMutation, toast])

  const toggle = useCallback(async () => {
    const newEnabled = !enabled
    setEnabled(newEnabled)

    try {
      await updateMutation.mutateAsync({
        data: {
          enabled: newEnabled,
          latitude: latitude ? parseFloat(latitude) : undefined,
          longitude: longitude ? parseFloat(longitude) : undefined,
          timezone: timezone || undefined,
          location_name: locationName || undefined,
        },
      })
      toast.success(newEnabled ? 'Location enabled' : 'Location disabled')
    } catch {
      setEnabled(!newEnabled)
      toast.error('Failed to update location')
    }
  }, [enabled, latitude, longitude, timezone, locationName, updateMutation, toast])

  const startChanging = useCallback(() => {
    setIsChanging(true)
    setLocationName('')
    setSearchQuery('')
  }, [])

  return {
    enabled,
    locationName,
    timezone,
    isChanging,
    isPending: updateMutation.isPending,

    // Search
    searchQuery,
    setSearchQuery,
    searchResults,
    isSearching,
    search,

    // Actions
    toggle,
    selectLocation,
    startChanging,
  }
}
