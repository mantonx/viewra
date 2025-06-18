declare module 'shaka-player/dist/shaka-player.ui.js' {
  interface ShakaPlayerConfig {
    streaming?: {
      rebufferingGoal?: number;
      bufferingGoal?: number;
      bufferBehind?: number;
      retryParameters?: {
        timeout?: number;
        maxAttempts?: number;
        baseDelay?: number;
        backoffFactor?: number;
        fuzzFactor?: number;
      };
    };
    manifest?: {
      retryParameters?: {
        timeout?: number;
        maxAttempts?: number;
        baseDelay?: number;
        backoffFactor?: number;
        fuzzFactor?: number;
      };
    };
  }

  interface ShakaErrorEvent {
    detail: {
      message?: string;
      category?: string;
      code?: number;
    };
  }

  interface ShakaTimeRange {
    start: number;
    end: number;
  }

  interface ShakaMediaInfo {
    streamingEngine?: unknown;
    minBufferTime?: number;
  }

  interface ShakaManifest {
    presentationTimeline?: {
      getDuration(): number;
      isLive(): boolean;
    };
  }

  interface ShakaPlayer {
    load(uri: string): Promise<void>;
    destroy(): Promise<void>;
    detach(): void;
    attach(videoElement: HTMLVideoElement): Promise<void>;
    configure(config: ShakaPlayerConfig): void;
    addEventListener(type: string, listener: (event: ShakaErrorEvent) => void): void;
    removeEventListener(type: string, listener: (event: ShakaErrorEvent) => void): void;
    getMediaInfo(): ShakaMediaInfo | null;
    seekRange(): ShakaTimeRange | null;
    getManifest(): ShakaManifest | null;
  }

  interface ShakaPolyfill {
    installAll(): void;
  }

  interface ShakaUIOverlayConstructor {
    new (player: ShakaPlayer, videoContainer: HTMLElement, videoElement: HTMLVideoElement): ShakaUIOverlayInstance;
  }
  
  interface ShakaUIOverlayInstance {
    destroy(): void;
  }

  interface ShakaUI {
    Overlay: ShakaUIOverlayConstructor;
  }

  interface ShakaPlayerStatic {
    new (): ShakaPlayer;
    new (videoElement?: HTMLVideoElement): ShakaPlayer;
    isBrowserSupported(): boolean;
  }

  interface Shaka {
    Player: ShakaPlayerStatic;
    polyfill: ShakaPolyfill;
    ui: ShakaUI;
  }

  const shaka: Shaka;
  export default shaka;
} 