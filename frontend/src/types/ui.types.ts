import type { MusicFile, SortField } from './music.types';

export interface ViewControlsProps {
  viewMode: 'grid' | 'list' | 'albums';
  filterText: string;
  filterGenre: string;
  sortField: SortField;
  sortDirection: 'asc' | 'desc';
  availableGenres: string[];
  onViewModeChange: (mode: 'grid' | 'list' | 'albums') => void;
  onFilterTextChange: (text: string) => void;
  onFilterGenreChange: (genre: string) => void;
  onSortChange: (field: SortField) => void;
  onSortDirectionToggle: () => void;
}

export interface MediaCardProps {
  variant: 'track' | 'album';
  item:
    | MusicFile
    | {
        title: string;
        year?: number;
        artwork?: string;
        tracks: MusicFile[];
      };
  isCurrentTrack?: boolean;
  isPlaying?: boolean;
  onPlay: (track: MusicFile) => void;
}
