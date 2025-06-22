import type { Meta, StoryObj } from '@storybook/react-vite';
import { ProgressBar } from './ProgressBar';

const meta = {
  title: 'MediaPlayer/ProgressBar',
  component: ProgressBar,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    currentTime: {
      control: { type: 'range', min: 0, max: 3600, step: 1 },
      description: 'Current playback time in seconds',
    },
    duration: {
      control: { type: 'range', min: 0, max: 3600, step: 1 },
      description: 'Total duration in seconds',
    },
    isSeekable: {
      control: 'boolean',
      description: 'Whether the progress bar is seekable',
    },
    isSeekingAhead: {
      control: 'boolean',
      description: 'Whether seek-ahead is active',
    },
    showTooltip: {
      control: 'boolean',
      description: 'Show time tooltip on hover',
    },
    showBuffered: {
      control: 'boolean',
      description: 'Show buffered ranges',
    },
    showSeekAheadIndicator: {
      control: 'boolean',
      description: 'Show seek-ahead indicator',
    },
    onSeek: {
      action: 'seek',
      description: 'Called when user seeks to a new position (0-1)',
    },
    onSeekIntent: {
      action: 'seekIntent',
      description: 'Called when user hovers over a position',
    },
  },
  decorators: [
    (Story) => (
      <div className="w-[800px] bg-slate-800 p-8 rounded">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ProgressBar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    currentTime: 120,
    duration: 600,
    bufferedRanges: [{ start: 0, end: 300 }],
    isSeekable: true,
    isSeekingAhead: false,
    showTooltip: true,
    showBuffered: true,
    showSeekAheadIndicator: true,
  },
};

export const Playing: Story = {
  args: {
    ...Default.args,
    currentTime: 245,
    bufferedRanges: [{ start: 0, end: 400 }],
  },
};

export const MultipleBufferedRanges: Story = {
  args: {
    ...Default.args,
    currentTime: 250,
    duration: 600,
    bufferedRanges: [
      { start: 0, end: 200 },
      { start: 240, end: 350 },
      { start: 400, end: 450 },
    ],
  },
};

export const SeekingAhead: Story = {
  args: {
    ...Default.args,
    isSeekingAhead: true,
    currentTime: 180,
    bufferedRanges: [{ start: 0, end: 200 }],
  },
};

export const NotSeekable: Story = {
  args: {
    ...Default.args,
    isSeekable: false,
    currentTime: 60,
  },
};

export const NoBuffering: Story = {
  args: {
    ...Default.args,
    bufferedRanges: [],
    currentTime: 30,
  },
};

export const FullyBuffered: Story = {
  args: {
    ...Default.args,
    currentTime: 300,
    duration: 600,
    bufferedRanges: [{ start: 0, end: 600 }],
  },
};

export const JustStarted: Story = {
  args: {
    ...Default.args,
    currentTime: 5,
    duration: 600,
    bufferedRanges: [{ start: 0, end: 60 }],
  },
};

export const NearEnd: Story = {
  args: {
    ...Default.args,
    currentTime: 590,
    duration: 600,
    bufferedRanges: [{ start: 0, end: 600 }],
  },
};

export const LongMovie: Story = {
  args: {
    ...Default.args,
    currentTime: 3600,
    duration: 7200,
    bufferedRanges: [
      { start: 0, end: 4800 },
      { start: 5000, end: 5500 },
    ],
  },
};

export const WithoutTooltip: Story = {
  args: {
    ...Default.args,
    showTooltip: false,
  },
};

export const WithoutBufferedDisplay: Story = {
  args: {
    ...Default.args,
    showBuffered: false,
  },
};

export const WithoutSeekAheadIndicator: Story = {
  args: {
    ...Default.args,
    showSeekAheadIndicator: false,
  },
};

export const LiveStream: Story = {
  args: {
    currentTime: 0,
    duration: 0,
    bufferedRanges: [],
    isSeekable: false,
    isSeekingAhead: false,
    showTooltip: false,
    showBuffered: false,
    showSeekAheadIndicator: false,
  },
};