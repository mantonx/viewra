import { vi } from 'vitest';
import type { MediaInfo, Episode, Movie } from '@/types/media';

// Mock media data
export const mockEpisode: Episode = {
  id: 1,
  seriesId: 1,
  seasonNumber: 1,
  episodeNumber: 1,
  title: 'Test Episode',
  overview: 'This is a test episode',
  airDate: '2023-01-01',
  runtime: 42,
  stillPath: '/test-still.jpg',
};

export const mockMovie: Movie = {
  id: 1,
  title: 'Test Movie',
  overview: 'This is a test movie',
  releaseDate: '2023-01-01',
  runtime: 120,
  posterPath: '/test-poster.jpg',
  backdropPath: '/test-backdrop.jpg',
  tmdbId: 12345,
  imdbId: 'tt1234567',
};

export const mockMediaInfo: MediaInfo = {
  id: 1,
  type: 'movie',
  title: 'Test Movie',
  description: 'This is a test movie',
  posterUrl: '/test-poster.jpg',
  backdropUrl: '/test-backdrop.jpg',
  year: '2023',
  duration: '2h 0m',
  playbackUrl: '/api/stream/test',
  subtitles: [],
};

// Mock Shaka Player
export const mockShakaPlayer = {
  load: vi.fn().mockResolvedValue(undefined),
  destroy: vi.fn(),
  configure: vi.fn(),
  getConfiguration: vi.fn().mockReturnValue({}),
  isLive: vi.fn().mockReturnValue(false),
  getManifestUri: vi.fn().mockReturnValue('test.mpd'),
  getVariantTracks: vi.fn().mockReturnValue([]),
  getTextTracks: vi.fn().mockReturnValue([]),
  selectTextTrack: vi.fn(),
  selectVariantTrack: vi.fn(),
  setTextTrackVisibility: vi.fn(),
};

// Mock HTMLMediaElement methods
export const mockVideoElement = {
  play: vi.fn().mockResolvedValue(undefined),
  pause: vi.fn(),
  load: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  currentTime: 0,
  duration: 120,
  volume: 1,
  muted: false,
  paused: true,
  ended: false,
  buffered: {
    length: 0,
    start: vi.fn().mockReturnValue(0),
    end: vi.fn().mockReturnValue(0),
  },
};

// Mock fetch for API calls
export const mockFetch = (data: any, status = 200) => {
  global.fetch = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => data,
    text: async () => JSON.stringify(data),
  });
};