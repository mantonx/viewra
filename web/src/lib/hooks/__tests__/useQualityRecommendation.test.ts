/**
 * Tests for useQualityRecommendation hook
 *
 * Note: These are basic unit tests. Full integration testing requires:
 * - Running backend server
 * - Mock capability detection APIs
 * - Browser environment for network detection
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { useQualityRecommendation } from '../useQualityRecommendation'
import * as adaptiveApi from '@/lib/api/adaptive'
import * as capabilityDetector from '@/lib/capabilities/CapabilityDetector'
import type { ClientCapabilities } from '@/lib/capabilities/types'
import type { QualityRecommendationResponse } from '@/lib/api/adaptive'

// Mock the API and capability detector
vi.mock('@/lib/api/adaptive')
vi.mock('@/lib/capabilities/CapabilityDetector')

describe('useQualityRecommendation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should initialize with correct default state', () => {
    const { result } = renderHook(() =>
      useQualityRecommendation({ skipAutoDetect: true })
    )

    expect(result.current.loading).toBe(false)
    expect(result.current.capabilities).toBe(null)
    expect(result.current.recommendation).toBe(null)
    expect(result.current.error).toBe(null)
    expect(result.current.isReady).toBe(false)
    expect(result.current.hasRecommendation).toBe(false)
  })

  it('should start loading when auto-detect is enabled', () => {
    const { result } = renderHook(() => useQualityRecommendation())

    expect(result.current.loading).toBe(true)
  })

  it('should detect capabilities and get recommendation', async () => {
    const mockCapabilities = {
      networkSpeedMbps: 50,
      deviceType: 'desktop',
      screenWidth: 1920,
      screenHeight: 1080,
      connectionType: 'wifi',
      isMetered: false,
      pixelRatio: 1,
      cpuCores: 8,
      memoryGB: 16,
      batteryLevel: 1,
      lowPowerMode: false,
      isCharging: true,
      supportedCodecs: ['h264', 'hevc'],
      hardwareAcceleration: true,
      maxDecodingProfile: '4k-60fps',
      userAgent: 'test',
      browserName: 'chrome',
      browserVersion: '120',
      detectedAt: new Date(),
    }

    const mockRecommendation = {
      height: 1080,
      width: 1920,
      displayName: 'Full HD',
      videoBitrate: 5000000,
      audioBitrate: 128000,
      preferredCodec: 'h264',
      reason: 'High-speed network and 1080p screen',
      score: 95.5,
      dataUsageMBPerHour: 2250,
    }

    vi.spyOn(capabilityDetector.capabilityDetector, 'detectCapabilities')
      .mockResolvedValue(mockCapabilities)

    vi.spyOn(adaptiveApi.adaptiveApi, 'recommendQuality')
      .mockResolvedValue(mockRecommendation)

    const { result } = renderHook(() => useQualityRecommendation())

    await waitFor(() => {
      expect(result.current.isReady).toBe(true)
    })

    expect(result.current.capabilities).toEqual(mockCapabilities)
    expect(result.current.recommendation).toEqual(mockRecommendation)
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBe(null)
    expect(result.current.hasRecommendation).toBe(true)
  })

  it('should handle errors gracefully', async () => {
    const mockError = new Error('Network error')

    vi.spyOn(capabilityDetector.capabilityDetector, 'detectCapabilities')
      .mockRejectedValue(mockError)

    const { result } = renderHook(() => useQualityRecommendation())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.error).toEqual(mockError)
    expect(result.current.isReady).toBe(false)
  })

  it('should allow manual refresh', async () => {
    vi.spyOn(capabilityDetector.capabilityDetector, 'detectCapabilities')
      .mockResolvedValue({} as ClientCapabilities)

    vi.spyOn(adaptiveApi.adaptiveApi, 'recommendQuality')
      .mockResolvedValue({} as QualityRecommendationResponse)

    const { result } = renderHook(() =>
      useQualityRecommendation({ skipAutoDetect: true })
    )

    expect(result.current.loading).toBe(false)

    result.current.refresh()

    expect(result.current.loading).toBe(true)
  })
})
