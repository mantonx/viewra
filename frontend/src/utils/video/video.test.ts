import {
  getBufferedRanges,
  getSeekableRanges,
  getTotalBuffered,
  getBufferedAhead,
  isTimeBuffered,
  getMaxBufferedTime,
  getMaxSeekableTime,
  detectDuration,
  canSeekTo,
  getReadyStateDescription,
  hasEnoughData,
  getNetworkStateDescription,
  getAspectRatio,
  isPlaying,
  getVideoQuality,
} from './video';

describe('Video Utils', () => {
  let mockVideo: Partial<HTMLVideoElement>;

  beforeEach(() => {
    mockVideo = {
      buffered: {
        length: 2,
        start: (index: number) => index === 0 ? 0 : 30,
        end: (index: number) => index === 0 ? 10 : 50,
      } as TimeRanges,
      seekable: {
        length: 1,
        start: () => 0,
        end: () => 100,
      } as TimeRanges,
      currentTime: 5,
      duration: 100,
      videoWidth: 1920,
      videoHeight: 1080,
      readyState: 4,
      networkState: 1,
      paused: false,
      ended: false,
    };
  });

  describe('getBufferedRanges', () => {
    it('should return buffered ranges', () => {
      const ranges = getBufferedRanges(mockVideo as HTMLVideoElement);
      expect(ranges).toEqual([
        { start: 0, end: 10 },
        { start: 30, end: 50 },
      ]);
    });

    it('should handle empty buffered', () => {
      mockVideo.buffered = { length: 0 } as TimeRanges;
      const ranges = getBufferedRanges(mockVideo as HTMLVideoElement);
      expect(ranges).toEqual([]);
    });
  });

  describe('getSeekableRanges', () => {
    it('should return seekable ranges', () => {
      const ranges = getSeekableRanges(mockVideo as HTMLVideoElement);
      expect(ranges).toEqual([{ start: 0, end: 100 }]);
    });
  });

  describe('getTotalBuffered', () => {
    it('should calculate total buffered time', () => {
      const total = getTotalBuffered(mockVideo as HTMLVideoElement);
      expect(total).toBe(30); // (10-0) + (50-30) = 30
    });
  });

  describe('getBufferedAhead', () => {
    it('should get buffered time ahead of current position', () => {
      const ahead = getBufferedAhead(mockVideo as HTMLVideoElement);
      expect(ahead).toBe(5); // current time 5, buffered until 10
    });

    it('should return 0 if current time not in buffered range', () => {
      mockVideo.currentTime = 25;
      const ahead = getBufferedAhead(mockVideo as HTMLVideoElement);
      expect(ahead).toBe(0);
    });
  });

  describe('isTimeBuffered', () => {
    it('should check if time is buffered', () => {
      expect(isTimeBuffered(mockVideo as HTMLVideoElement, 5)).toBe(true);
      expect(isTimeBuffered(mockVideo as HTMLVideoElement, 35)).toBe(true);
      expect(isTimeBuffered(mockVideo as HTMLVideoElement, 25)).toBe(false);
    });
  });

  describe('getMaxBufferedTime', () => {
    it('should get maximum buffered end time', () => {
      const max = getMaxBufferedTime(mockVideo as HTMLVideoElement);
      expect(max).toBe(50);
    });
  });

  describe('getMaxSeekableTime', () => {
    it('should get maximum seekable end time', () => {
      const max = getMaxSeekableTime(mockVideo as HTMLVideoElement);
      expect(max).toBe(100);
    });
  });

  describe('detectDuration', () => {
    it('should detect duration from video.duration', () => {
      const duration = detectDuration(mockVideo as HTMLVideoElement);
      expect(duration).toBe(100);
    });

    it('should fallback to seekable range', () => {
      mockVideo.duration = NaN;
      const duration = detectDuration(mockVideo as HTMLVideoElement);
      expect(duration).toBe(100);
    });

    it('should fallback to buffered range', () => {
      mockVideo.duration = NaN;
      mockVideo.seekable = { length: 0 } as TimeRanges;
      const duration = detectDuration(mockVideo as HTMLVideoElement);
      expect(duration).toBe(50);
    });
  });

  describe('canSeekTo', () => {
    it('should check if time is seekable', () => {
      expect(canSeekTo(mockVideo as HTMLVideoElement, 50)).toBe(true);
      expect(canSeekTo(mockVideo as HTMLVideoElement, 150)).toBe(false);
    });
  });

  describe('getReadyStateDescription', () => {
    it('should return ready state descriptions', () => {
      expect(getReadyStateDescription(0)).toBe('HAVE_NOTHING');
      expect(getReadyStateDescription(1)).toBe('HAVE_METADATA');
      expect(getReadyStateDescription(2)).toBe('HAVE_CURRENT_DATA');
      expect(getReadyStateDescription(3)).toBe('HAVE_FUTURE_DATA');
      expect(getReadyStateDescription(4)).toBe('HAVE_ENOUGH_DATA');
      expect(getReadyStateDescription(99)).toBe('UNKNOWN');
    });
  });

  describe('hasEnoughData', () => {
    it('should check if video has enough data', () => {
      expect(hasEnoughData(mockVideo as HTMLVideoElement)).toBe(true);
      
      mockVideo.readyState = 2;
      expect(hasEnoughData(mockVideo as HTMLVideoElement)).toBe(false);
    });
  });

  describe('getNetworkStateDescription', () => {
    it('should return network state descriptions', () => {
      expect(getNetworkStateDescription(0)).toBe('NETWORK_EMPTY');
      expect(getNetworkStateDescription(1)).toBe('NETWORK_IDLE');
      expect(getNetworkStateDescription(2)).toBe('NETWORK_LOADING');
      expect(getNetworkStateDescription(3)).toBe('NETWORK_NO_SOURCE');
      expect(getNetworkStateDescription(99)).toBe('UNKNOWN');
    });
  });

  describe('getAspectRatio', () => {
    it('should calculate aspect ratio', () => {
      const ratio = getAspectRatio(mockVideo as HTMLVideoElement);
      expect(ratio).toBeCloseTo(1.778, 3); // 1920/1080
    });

    it('should return default ratio for invalid dimensions', () => {
      mockVideo.videoWidth = 0;
      mockVideo.videoHeight = 0;
      const ratio = getAspectRatio(mockVideo as HTMLVideoElement);
      expect(ratio).toBeCloseTo(1.778, 3); // 16/9
    });
  });

  describe('isPlaying', () => {
    it('should check if video is playing', () => {
      expect(isPlaying(mockVideo as HTMLVideoElement)).toBe(true);
      
      mockVideo.paused = true;
      expect(isPlaying(mockVideo as HTMLVideoElement)).toBe(false);
      
      mockVideo.paused = false;
      mockVideo.ended = true;
      expect(isPlaying(mockVideo as HTMLVideoElement)).toBe(false);
      
      mockVideo.ended = false;
      mockVideo.readyState = 1;
      expect(isPlaying(mockVideo as HTMLVideoElement)).toBe(false);
    });
  });

  describe('getVideoQuality', () => {
    it('should return video quality info', () => {
      const quality = getVideoQuality(mockVideo as HTMLVideoElement);
      expect(quality).toEqual({
        width: 1920,
        height: 1080,
        resolution: '1920x1080',
      });
    });
  });
});