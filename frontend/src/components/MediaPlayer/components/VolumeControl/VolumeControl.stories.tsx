import type { Meta, StoryObj } from '@storybook/react-vite';
import { VolumeControl } from './VolumeControl';

const meta = {
  title: 'MediaPlayer/VolumeControl',
  component: VolumeControl,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    volume: {
      control: { type: 'range', min: 0, max: 1, step: 0.1 },
      description: 'Current volume level (0-1)',
    },
    isMuted: {
      control: 'boolean',
      description: 'Whether audio is muted',
    },
    onVolumeChange: {
      action: 'volumeChange',
      description: 'Called when volume changes',
    },
    onToggleMute: {
      action: 'toggleMute',
      description: 'Called when mute is toggled',
    },
  },
  decorators: [
    (Story) => (
      <div className="bg-slate-800 p-8 rounded">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof VolumeControl>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    volume: 0.7,
    isMuted: false,
  },
};

export const Muted: Story = {
  args: {
    volume: 0.7,
    isMuted: true,
  },
};

export const FullVolume: Story = {
  args: {
    volume: 1,
    isMuted: false,
  },
};

export const LowVolume: Story = {
  args: {
    volume: 0.2,
    isMuted: false,
  },
};

export const Silent: Story = {
  args: {
    volume: 0,
    isMuted: false,
  },
};

export const HalfVolume: Story = {
  args: {
    volume: 0.5,
    isMuted: false,
  },
};