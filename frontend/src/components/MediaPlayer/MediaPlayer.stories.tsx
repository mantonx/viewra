import type { Meta, StoryObj } from '@storybook/react-vite';
import { MediaPlayer } from './MediaPlayer';
import { MediaPlayerDemo } from './MediaPlayerDemo';

// For the real MediaPlayer (shows loading state)
const realMeta = {
  title: 'MediaPlayer/MediaPlayer (Real)',
  component: MediaPlayer,
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: 'The actual MediaPlayer component. Shows loading state in Storybook since it requires backend integration.',
      },
    },
  },
  tags: ['autodocs'],
  argTypes: {
    mediaType: {
      control: 'select',
      options: ['episode', 'movie'],
      description: 'Type of media to play',
    },
    autoplay: {
      control: 'boolean',
      description: 'Whether to start playing automatically',
    },
    onBack: {
      action: 'back clicked',
      description: 'Callback when back button is clicked',
    },
  },
};

// For the demo MediaPlayer (fully functional UI)
const meta = {
  title: 'MediaPlayer/MediaPlayer',
  component: MediaPlayerDemo,
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
A fully functional demo of the MediaPlayer component that works without backend integration. 

## Features:
- **Playback Controls**: Play/pause, stop, restart, skip forward/backward
- **Progress Bar**: Seekable with hover preview and buffered ranges
- **Volume Control**: Adjustable volume with mute toggle
- **Time Display**: Current time and remaining time
- **Media Info**: Shows episode/movie metadata
- **Auto-hide Controls**: Controls hide after 3 seconds of inactivity
- **Keyboard Shortcuts**: Space (play/pause), arrows (seek), M (mute), F (fullscreen)

## Interactive Elements:
- Click the play button to start/pause playback
- Click or drag on the progress bar to seek
- Hover over the progress bar to see time preview
- Adjust volume with the slider
- Controls auto-hide when playing (move mouse to show)
        `,
      },
    },
  },
  tags: ['autodocs'],
  argTypes: {
    mediaType: {
      control: 'select',
      options: ['episode', 'movie'],
      description: 'Type of media to play',
    },
    autoplay: {
      control: 'boolean',
      description: 'Whether to start playing automatically',
    },
    initialTime: {
      control: { type: 'range', min: 0, max: 7200, step: 30 },
      description: 'Initial playback time in seconds',
    },
    showBuffering: {
      control: 'boolean',
      description: 'Show buffering overlay',
    },
    showError: {
      control: 'boolean',
      description: 'Show error state',
    },
    errorMessage: {
      control: 'text',
      description: 'Error message to display',
    },
    onBack: {
      action: 'back clicked',
      description: 'Callback when back button is clicked',
    },
  },
} satisfies Meta<typeof MediaPlayerDemo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const EpisodePlayer: Story = {
  args: {
    mediaType: 'episode',
    autoplay: false,
    initialTime: 300, // 5 minutes in
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Episode player showing "The Beginning" (S01E01) from "Epic Adventure Series". Duration: 45 minutes.',
      },
    },
  },
};

export const MoviePlayer: Story = {
  args: {
    mediaType: 'movie',
    autoplay: false,
    initialTime: 1200, // 20 minutes in
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Movie player showing "The Great Adventure". Duration: 2 hours.',
      },
    },
  },
};

export const AutoplayEnabled: Story = {
  args: {
    mediaType: 'episode',
    autoplay: true,
    initialTime: 0,
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Episode that starts playing automatically. The timer will advance and controls will auto-hide.',
      },
    },
  },
};

export const Paused: Story = {
  args: {
    mediaType: 'movie',
    autoplay: false,
    initialTime: 3600, // 1 hour in
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Movie paused at the 1-hour mark.',
      },
    },
  },
};

export const BufferingState: Story = {
  args: {
    mediaType: 'episode',
    autoplay: false,
    initialTime: 600,
    showBuffering: true,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Player showing buffering spinner overlay.',
      },
    },
  },
};

export const ErrorState: Story = {
  args: {
    mediaType: 'movie',
    autoplay: false,
    initialTime: 0,
    showBuffering: false,
    showError: true,
    errorMessage: 'Unable to connect to streaming server. Please check your connection and try again.',
  },
  parameters: {
    docs: {
      description: {
        story: 'Player showing error state with retry button.',
      },
    },
  },
};

export const JustStarted: Story = {
  args: {
    mediaType: 'episode',
    autoplay: false,
    initialTime: 5,
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Episode just started playing (5 seconds in).',
      },
    },
  },
};

export const NearEnd: Story = {
  args: {
    mediaType: 'movie',
    autoplay: false,
    initialTime: 7000, // Near end of 2-hour movie
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Movie near the end (less than 4 minutes remaining).',
      },
    },
  },
};

export const MidEpisode: Story = {
  args: {
    mediaType: 'episode',
    autoplay: false,
    initialTime: 1350, // 22.5 minutes (middle of 45-min episode)
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Episode at the halfway point.',
      },
    },
  },
};