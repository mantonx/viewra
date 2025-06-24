import type { Meta, StoryObj } from '@storybook/react-vite';
import { MediaPlayerDemo } from './MediaPlayerDemo';
import '@/styles/player-theme.css';

const meta: Meta<typeof MediaPlayerDemo> = {
  title: 'MediaPlayer/MediaPlayer (Demo)',
  component: MediaPlayerDemo,
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
Demo of the VidstackPlayer component styling and interaction patterns.

## About MediaPlayer:
The MediaPlayer is a modern video player built with Vidstack and designed for Viewra. It supports both DASH and HLS streaming with device-specific optimizations. This demo shows the UI and theming without requiring backend integration.

## Features Demonstrated:
- **Design System**: Player-specific design tokens from player-theme.css
- **Interactive Controls**: Play/pause, seek, volume, fullscreen controls
- **Auto-hide Behavior**: Controls hide automatically during playback
- **Responsive Design**: Adapts to different screen sizes
- **Loading States**: Buffering and error state handling
- **Media Info**: Episode/movie metadata display

## Real Implementation Features:
- **Adaptive Streaming**: Supports both DASH (desktop) and HLS (iOS/Safari) with automatic format selection
- **Session Tracking**: Built-in analytics and session tracking
- **Device Profiles**: Automatic device capability detection
- **Backend Integration**: Media metadata and transcoding session management

## Design Tokens:
- CSS custom properties for consistent theming
- Player-specific gradients and accent colors
- Smooth transitions and hover effects
- Accessible focus states
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
    className: {
      control: 'text',
      description: 'Additional CSS classes',
    },
    onBack: {
      action: 'back clicked',
      description: 'Callback when back button is clicked',
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const EpisodePlayer: Story = {
  args: {
    mediaType: 'episode',
    autoplay: false,
    initialTime: 300,
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: `
Episode player demo showing "The Beginning" (S01E01) from "Epic Adventure Series". 
Duration: 45 minutes, starting 5 minutes in.

**Features demonstrated:**
- Episode metadata display
- Player accent theming from design tokens
- Auto-hide controls behavior
- Responsive progress bar with buffered ranges
- Vidstack-powered playback engine
        `,
      },
    },
  },
};

export const MoviePlayer: Story = {
  args: {
    mediaType: 'movie',
    autoplay: false,
    initialTime: 1200,
    showBuffering: false,
    showError: false,
  },
  parameters: {
    docs: {
      description: {
        story: `
Movie player demo showing "The Great Adventure". Duration: 2 hours, starting 20 minutes in.

**Movie-specific features:**
- Movie metadata display (no episode/season info)
- Longer duration handling
- Different media info overlay content
        `,
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
        story: `
Episode that starts playing automatically. The timer advances and controls auto-hide after 3 seconds.

**Autoplay demo features:**
- Starts playing immediately
- Controls auto-hide during playback
- Progress bar updates in real-time
- Time display shows current/remaining time
        `,
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
        story: `
Player showing buffering state with spinner overlay.

**Buffering features:**
- Animated loading spinner
- Overlay with themed background
- Maintains player controls visibility
- Proper accessibility labels
        `,
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
        story: `
Player showing error state with retry button.

**Error state features:**
- Clear error message display
- Styled retry button with player theme
- Proper error handling UI
- Accessibility-friendly error states
        `,
      },
    },
  },
};

export const WithCustomStyles: Story = {
  args: {
    mediaType: 'movie',
    autoplay: false,
    initialTime: 3600,
    showBuffering: false,
    showError: false,
    className: 'border-4 border-purple-500',
  },
  parameters: {
    docs: {
      description: {
        story: `
Player with custom styling applied. Shows how additional classes can be added 
while maintaining the core player theme.

**Custom styling:**
- Additional border added via className
- Player theme tokens still applied
- Maintains responsive behavior
- Movie at 1-hour mark
        `,
      },
    },
  },
};