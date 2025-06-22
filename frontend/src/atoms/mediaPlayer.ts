import { atom } from 'jotai';
import type { MediaItem, MediaFile, PlaybackDecision, PlayerState, SessionState, SeekAheadState, ProgressState } from '../components/MediaPlayer/types';
import type shaka from 'shaka-player';

export const currentMediaAtom = atom<MediaItem | null>(null);

export const mediaFileAtom = atom<MediaFile | null>(null);

export const playbackDecisionAtom = atom<PlaybackDecision | null>(null);

export const playerStateAtom = atom<PlayerState>({
  isPlaying: false,
  duration: 0,
  currentTime: 0,
  volume: 1,
  isMuted: false,
  isFullscreen: false,
  isBuffering: false,
  isSeekingAhead: false,
  showControls: true,
});

export const sessionStateAtom = atom<SessionState>({
  activeSessions: new Set<string>(),
  isStoppingSession: false,
});

export const seekAheadStateAtom = atom<SeekAheadState>({
  isSeekingAhead: false,
  seekOffset: 0,
});

export const progressStateAtom = atom<ProgressState>({
  seekableDuration: 0,
  originalDuration: 0,
  hoverTime: null,
});

export const loadingStateAtom = atom<{
  isLoading: boolean;
  error: string | null;
  isVideoLoading: boolean;
}>({
  isLoading: true,
  error: null,
  isVideoLoading: true,
});

export const configAtom = atom<{
  debug: boolean;
  autoplay: boolean;
  startTime: number;
}>({
  debug: false,
  autoplay: true,
  startTime: 0,
});

export const debugAtom = atom<boolean>(false);

export const activeSessionsAtom = atom<Set<string>>(new Set<string>());

export const playerInitializedAtom = atom<boolean>(false);

export const videoElementAtom = atom<HTMLVideoElement | null>(null);

export const shakaPlayerAtom = atom<shaka.Player | null>(null);

export const shakaUIAtom = atom<shaka.ui.Overlay | null>(null);

export const currentTimeAtom = atom(
  (get) => get(playerStateAtom).currentTime,
  (get, set, newTime: number) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, currentTime: newTime });
  }
);

export const isPlayingAtom = atom(
  (get) => get(playerStateAtom).isPlaying,
  (get, set, isPlaying: boolean) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, isPlaying });
  }
);

export const volumeAtom = atom(
  (get) => get(playerStateAtom).volume,
  (get, set, volume: number) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, volume });
  }
);

export const isMutedAtom = atom(
  (get) => get(playerStateAtom).isMuted,
  (get, set, isMuted: boolean) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, isMuted });
  }
);

export const isFullscreenAtom = atom(
  (get) => get(playerStateAtom).isFullscreen,
  (get, set, isFullscreen: boolean) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, isFullscreen });
  }
);

export const showControlsAtom = atom(
  (get) => get(playerStateAtom).showControls,
  (get, set, showControls: boolean) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, showControls });
  }
);

export const durationAtom = atom(
  (get) => get(playerStateAtom).duration,
  (get, set, duration: number) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, duration });
  }
);

export const isBufferingAtom = atom(
  (get) => get(playerStateAtom).isBuffering,
  (get, set, isBuffering: boolean) => {
    const playerState = get(playerStateAtom);
    set(playerStateAtom, { ...playerState, isBuffering });
  }
);