import type { Meta, StoryObj } from '@storybook/react-vite';
import { StatusOverlay } from './StatusOverlay';

const meta = {
  title: 'MediaPlayer/StatusOverlay',
  component: StatusOverlay,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    isBuffering: {
      control: 'boolean',
      description: 'Whether media is buffering',
    },
    isSeekingAhead: {
      control: 'boolean',
      description: 'Whether seek-ahead is active',
    },
    isLoading: {
      control: 'boolean',
      description: 'Whether media is loading',
    },
    error: {
      control: 'text',
      description: 'Error message to display',
    },
    showPlaybackInfo: {
      control: 'boolean',
      description: 'Whether to show playback info (debug mode)',
    },
  },
  decorators: [
    (Story) => (
      <div className="w-[800px] h-[450px] bg-slate-900 relative rounded-lg overflow-hidden">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StatusOverlay>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: false,
    error: null,
    showPlaybackInfo: false,
  },
};

export const Loading: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: true,
    error: null,
  },
};

export const Buffering: Story = {
  args: {
    isBuffering: true,
    isSeekingAhead: false,
    isLoading: false,
    error: null,
  },
};

export const SeekingAhead: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: true,
    isLoading: false,
    error: null,
  },
};

export const Error: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: false,
    error: 'Failed to load media. Please try again.',
  },
};

export const NetworkError: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: false,
    error: 'Network error: Unable to connect to server',
  },
};

export const CodecError: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: false,
    error: 'Unsupported video codec. Transcoding required.',
  },
};

export const WithTranscodingInfo: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: false,
    error: null,
    playbackInfo: {
      isTranscoding: true,
      reason: 'Video codec not supported by browser',
      sessionCount: 1,
    },
    showPlaybackInfo: true,
  },
};

export const WithDirectPlayInfo: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: false,
    error: null,
    playbackInfo: {
      isTranscoding: false,
      reason: 'Direct play supported',
      sessionCount: 0,
    },
    showPlaybackInfo: true,
  },
};

export const MultipleStates: Story = {
  args: {
    isBuffering: true,
    isSeekingAhead: true,
    isLoading: false,
    error: null,
  },
};

export const TranscodingWithMultipleSessions: Story = {
  args: {
    isBuffering: false,
    isSeekingAhead: false,
    isLoading: false,
    error: null,
    playbackInfo: {
      isTranscoding: true,
      reason: 'HEVC codec requires transcoding',
      sessionCount: 3,
    },
    showPlaybackInfo: true,
  },
};