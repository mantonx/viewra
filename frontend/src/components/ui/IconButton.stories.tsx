import type { Meta, StoryObj } from '@storybook/react-vite';
import IconButton from './IconButton';
import { Play, Pause, SkipForward, SkipBack, Volume2, VolumeX, Maximize } from 'lucide-react';

const meta = {
  title: 'UI/IconButton',
  component: IconButton,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    icon: {
      control: false,
      description: 'Icon component to display',
    },
    label: {
      control: 'text',
      description: 'Accessibility label',
    },
    onClick: {
      action: 'clicked',
      description: 'Click handler',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether button is disabled',
    },
    className: {
      control: 'text',
      description: 'Additional CSS classes',
    },
    size: {
      control: 'select',
      options: ['sm', 'md', 'lg'],
      description: 'Button size',
    },
  },
  decorators: [
    (Story) => (
      <div className="flex items-center gap-4 p-8 bg-slate-800 rounded">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof IconButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const PlayButton: Story = {
  args: {
    icon: Play,
    label: 'Play',
  },
};

export const PauseButton: Story = {
  args: {
    icon: Pause,
    label: 'Pause',
  },
};

export const SkipForwardButton: Story = {
  args: {
    icon: SkipForward,
    label: 'Skip Forward',
  },
};

export const SkipBackButton: Story = {
  args: {
    icon: SkipBack,
    label: 'Skip Back',
  },
};

export const VolumeButton: Story = {
  args: {
    icon: Volume2,
    label: 'Volume',
  },
};

export const MutedButton: Story = {
  args: {
    icon: VolumeX,
    label: 'Muted',
  },
};

export const FullscreenButton: Story = {
  args: {
    icon: Maximize,
    label: 'Fullscreen',
  },
};

export const Disabled: Story = {
  args: {
    icon: Play,
    label: 'Play',
    disabled: true,
  },
};

export const Small: Story = {
  args: {
    icon: Play,
    label: 'Play',
    size: 'sm',
  },
};

export const Large: Story = {
  args: {
    icon: Play,
    label: 'Play',
    size: 'lg',
  },
};

export const CustomStyle: Story = {
  args: {
    icon: Play,
    label: 'Play',
    className: 'text-purple-500 hover:text-purple-400',
  },
};

export const ButtonGroup: Story = {
  render: () => (
    <>
      <IconButton icon={SkipBack} label="Skip Back" />
      <IconButton icon={Play} label="Play" />
      <IconButton icon={SkipForward} label="Skip Forward" />
    </>
  ),
};