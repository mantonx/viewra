import type { Meta, StoryObj } from '@storybook/react-vite';
import { VideoControls } from './VideoControls';

const meta = {
  title: 'MediaPlayer/VideoControls',
  component: VideoControls,
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
  argTypes: {
    isPlaying: {
      control: 'boolean',
      description: 'Whether video is playing',
    },
    currentTime: {
      control: { type: 'range', min: 0, max: 3600, step: 1 },
      description: 'Current playback time in seconds',
    },
    duration: {
      control: { type: 'range', min: 0, max: 3600, step: 1 },
      description: 'Total duration in seconds',
    },
    volume: {
      control: { type: 'range', min: 0, max: 1, step: 0.1 },
      description: 'Current volume level (0-1)',
    },
    isMuted: {
      control: 'boolean',
      description: 'Whether audio is muted',
    },
    isFullscreen: {
      control: 'boolean',
      description: 'Whether in fullscreen mode',
    },
    isSeekingAhead: {
      control: 'boolean',
      description: 'Whether seek-ahead is active',
    },
    showStopButton: {
      control: 'boolean',
      description: 'Show stop button',
    },
    showSkipButtons: {
      control: 'boolean',
      description: 'Show skip forward/backward buttons',
    },
    showVolumeControl: {
      control: 'boolean',
      description: 'Show volume control',
    },
    showFullscreenButton: {
      control: 'boolean',
      description: 'Show fullscreen button',
    },
    showTimeDisplay: {
      control: 'boolean',
      description: 'Show time display',
    },
    skipSeconds: {
      control: { type: 'range', min: 5, max: 30, step: 5 },
      description: 'Seconds to skip forward/backward',
    },
    onPlayPause: { action: 'playPause' },
    onStop: { action: 'stop' },
    onRestart: { action: 'restart' },
    onSeek: { action: 'seek' },
    onSeekIntent: { action: 'seekIntent' },
    onSkipBackward: { action: 'skipBackward' },
    onSkipForward: { action: 'skipForward' },
    onVolumeChange: { action: 'volumeChange' },
    onToggleMute: { action: 'toggleMute' },
    onToggleFullscreen: { action: 'toggleFullscreen' },
  },
  decorators: [
    (Story) => (
      <div className="h-screen bg-slate-900 flex items-end p-8">
        <div className="w-full max-w-4xl mx-auto">
          <Story />
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof VideoControls>;

export default meta;
type Story = StoryObj<typeof meta>;

// Helper to create buffered ranges
const createBufferedRanges = (duration: number, bufferedPercent: number) => {
  const bufferedEnd = (duration * bufferedPercent) / 100;
  return [{ start: 0, end: bufferedEnd }];
};

export const Default: Story = {
  args: {
    isPlaying: false,
    currentTime: 120,
    duration: 600,
    bufferedRanges: createBufferedRanges(600, 50),
    volume: 0.7,
    isMuted: false,
    isFullscreen: false,
    isSeekingAhead: false,
    showStopButton: true,
    showSkipButtons: true,
    showVolumeControl: true,
    showFullscreenButton: true,
    showTimeDisplay: true,
    skipSeconds: 10,
  },
};

export const Playing: Story = {
  args: {
    ...Default.args,
    isPlaying: true,
  },
};

export const Paused: Story = {
  args: {
    ...Default.args,
    isPlaying: false,
    currentTime: 300,
  },
};

export const Muted: Story = {
  args: {
    ...Default.args,
    isMuted: true,
  },
};

export const Fullscreen: Story = {
  args: {
    ...Default.args,
    isFullscreen: true,
  },
};

export const SeekingAhead: Story = {
  args: {
    ...Default.args,
    isSeekingAhead: true,
    currentTime: 400,
    bufferedRanges: createBufferedRanges(600, 40),
  },
};

export const MinimalControls: Story = {
  args: {
    ...Default.args,
    showStopButton: false,
    showSkipButtons: false,
    showVolumeControl: false,
    showFullscreenButton: false,
  },
};

export const WithoutTimeDisplay: Story = {
  args: {
    ...Default.args,
    showTimeDisplay: false,
  },
};

export const CustomSkipSeconds: Story = {
  args: {
    ...Default.args,
    skipSeconds: 30,
  },
};

export const AtStart: Story = {
  args: {
    ...Default.args,
    currentTime: 0,
    bufferedRanges: createBufferedRanges(600, 10),
  },
};

export const NearEnd: Story = {
  args: {
    ...Default.args,
    currentTime: 590,
    duration: 600,
    bufferedRanges: createBufferedRanges(600, 100),
  },
};

export const LongMovie: Story = {
  args: {
    ...Default.args,
    currentTime: 3600,
    duration: 7200,
    bufferedRanges: createBufferedRanges(7200, 60),
  },
};

export const MultipleBufferedRanges: Story = {
  args: {
    ...Default.args,
    currentTime: 250,
    bufferedRanges: [
      { start: 0, end: 200 },
      { start: 240, end: 350 },
      { start: 400, end: 450 },
    ],
  },
};

export const NoBuffer: Story = {
  args: {
    ...Default.args,
    bufferedRanges: [],
    currentTime: 60,
  },
};