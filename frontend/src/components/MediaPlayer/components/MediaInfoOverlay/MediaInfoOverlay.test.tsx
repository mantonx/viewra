import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { MediaInfoOverlay } from './MediaInfoOverlay';
import type { Episode, Movie } from '../../types';

// Mock the isEpisode utility
vi.mock('@/utils/mediaValidation', () => ({
  isEpisode: (media: any) => media?.type === 'episode',
}));

describe('MediaInfoOverlay', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const mockMovie: Movie = {
    id: '1',
    type: 'movie',
    title: 'Test Movie',
    description: 'A great test movie',
    release_date: '2023-01-01',
    runtime: 120,
  };

  const mockEpisode: Episode = {
    id: '2',
    type: 'episode',
    title: 'Test Episode',
    description: 'An exciting episode',
    episode_number: 5,
    season_number: 2,
    series: {
      id: 'series-1',
      title: 'Test Series',
      description: 'A test series',
    },
  };

  it('renders nothing when media is null', () => {
    const { container } = render(<MediaInfoOverlay media={null} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders movie information correctly', () => {
    const { getByText } = render(<MediaInfoOverlay media={mockMovie} />);
    
    expect(getByText('Test Movie')).toBeInTheDocument();
    expect(getByText('2023')).toBeInTheDocument();
    expect(getByText('A great test movie')).toBeInTheDocument();
  });

  it('renders episode information correctly', () => {
    const { getByText } = render(<MediaInfoOverlay media={mockEpisode} />);
    
    expect(getByText('Test Series')).toBeInTheDocument();
    expect(getByText('Season 2, Episode 5')).toBeInTheDocument();
    expect(getByText('Test Episode')).toBeInTheDocument();
    expect(getByText('An exciting episode')).toBeInTheDocument();
  });

  it('handles missing movie description', () => {
    const movieWithoutDescription = { ...mockMovie, description: undefined };
    const { queryByText } = render(<MediaInfoOverlay media={movieWithoutDescription} />);
    
    expect(queryByText('A great test movie')).not.toBeInTheDocument();
  });

  it('handles missing episode description', () => {
    const episodeWithoutDescription = { ...mockEpisode, description: undefined };
    const { queryByText } = render(<MediaInfoOverlay media={episodeWithoutDescription} />);
    
    expect(queryByText('An exciting episode')).not.toBeInTheDocument();
  });

  it('handles missing release date for movie', () => {
    const movieWithoutDate = { ...mockMovie, release_date: undefined };
    const { container } = render(<MediaInfoOverlay media={movieWithoutDate} />);
    
    // Should not show year
    expect(container.textContent).not.toMatch(/\d{4}/);
  });

  it('applies correct position classes', () => {
    const positions = [
      { position: 'top-left' as const, expectedClass: 'top-4 left-4' },
      { position: 'top-right' as const, expectedClass: 'top-4 right-4' },
      { position: 'bottom-left' as const, expectedClass: 'bottom-4 left-4' },
      { position: 'bottom-right' as const, expectedClass: 'bottom-4 right-4' },
    ];

    positions.forEach(({ position, expectedClass }) => {
      const { container } = render(
        <MediaInfoOverlay media={mockMovie} position={position} />
      );
      
      const overlay = container.firstChild;
      expect(overlay).toHaveClass(...expectedClass.split(' '));
    });
  });

  it('auto-hides after specified delay', async () => {
    const { container } = render(
      <MediaInfoOverlay media={mockMovie} autoHide={true} autoHideDelay={2000} />
    );

    const overlay = container.firstChild;
    
    // Should be visible initially
    expect(overlay).toHaveClass('opacity-100');
    expect(overlay).not.toHaveClass('opacity-0');

    // Fast forward time
    vi.advanceTimersByTime(2000);

    await waitFor(() => {
      expect(overlay).toHaveClass('opacity-0');
      expect(overlay).not.toHaveClass('opacity-100');
    });
  });

  it('does not auto-hide when autoHide is false', () => {
    const { container } = render(
      <MediaInfoOverlay media={mockMovie} autoHide={false} />
    );

    const overlay = container.firstChild;
    
    // Should remain visible
    vi.advanceTimersByTime(10000);
    
    expect(overlay).toHaveClass('opacity-100');
    expect(overlay).not.toHaveClass('opacity-0');
  });

  it('applies showOnHover styles', () => {
    const { container } = render(
      <MediaInfoOverlay media={mockMovie} showOnHover={true} autoHide={false} />
    );

    const overlay = container.firstChild;
    expect(overlay).toHaveClass('hover:opacity-100');
  });

  it('applies custom className', () => {
    const { container } = render(
      <MediaInfoOverlay media={mockMovie} className="custom-class" />
    );

    const overlay = container.firstChild;
    expect(overlay).toHaveClass('custom-class');
  });

  it('clears timeout on unmount', () => {
    const clearTimeoutSpy = vi.spyOn(global, 'clearTimeout');
    
    const { unmount } = render(
      <MediaInfoOverlay media={mockMovie} autoHide={true} />
    );

    unmount();

    expect(clearTimeoutSpy).toHaveBeenCalled();
  });

  it('resets visibility when media changes', async () => {
    const { rerender, container } = render(
      <MediaInfoOverlay media={mockMovie} autoHide={true} autoHideDelay={1000} />
    );

    // Fast forward to hide
    vi.advanceTimersByTime(1000);

    await waitFor(() => {
      expect(container.firstChild).toHaveClass('opacity-0');
    });

    // Change media
    rerender(
      <MediaInfoOverlay media={mockEpisode} autoHide={true} autoHideDelay={1000} />
    );

    // Should be visible again
    expect(container.firstChild).toHaveClass('opacity-100');
  });

  it('truncates long descriptions with line-clamp', () => {
    const longDescription = 'This is a very long description. '.repeat(10);
    const movieWithLongDesc = { ...mockMovie, description: longDescription };
    
    const { container } = render(<MediaInfoOverlay media={movieWithLongDesc} />);
    
    const descElement = container.querySelector('.line-clamp-2');
    expect(descElement).toBeInTheDocument();
  });

  it('handles series without additional metadata', () => {
    const minimalEpisode: Episode = {
      id: '3',
      type: 'episode',
      title: 'Minimal Episode',
      episode_number: 1,
      season_number: 1,
      series: {
        id: 'series-2',
        title: 'Minimal Series',
      },
    };

    const { getByText, queryByText } = render(
      <MediaInfoOverlay media={minimalEpisode} />
    );
    
    expect(getByText('Minimal Series')).toBeInTheDocument();
    expect(getByText('Season 1, Episode 1')).toBeInTheDocument();
    expect(getByText('Minimal Episode')).toBeInTheDocument();
    
    // No description should be shown
    const descElements = queryByText(/line-clamp-2/);
    expect(descElements).not.toBeInTheDocument();
  });
});