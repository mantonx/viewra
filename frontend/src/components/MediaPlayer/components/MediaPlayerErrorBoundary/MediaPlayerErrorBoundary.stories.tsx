import type { Meta, StoryObj } from '@storybook/react-vite';
import { MediaPlayerErrorBoundary } from './MediaPlayerErrorBoundary';
import { useState } from 'react';

const meta = {
  title: 'Components/MediaPlayer/MediaPlayerErrorBoundary',
  component: MediaPlayerErrorBoundary,
  parameters: {
    layout: 'fullscreen',
  },
} satisfies Meta<typeof MediaPlayerErrorBoundary>;

export default meta;
type Story = StoryObj<typeof meta>;

// Component that throws an error
const ThrowError = ({ error }: { error: string }) => {
  throw new Error(error);
};

// Component that throws an error on button click
const ErrorButton = ({ errorMessage }: { errorMessage: string }) => {
  const [shouldError, setShouldError] = useState(false);

  if (shouldError) {
    throw new Error(errorMessage);
  }

  return (
    <div className="flex items-center justify-center h-screen bg-gray-900">
      <button
        onClick={() => setShouldError(true)}
        className="px-6 py-3 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
      >
        Click to Trigger Error
      </button>
    </div>
  );
};

// Working media player mock
const WorkingPlayer = () => (
  <div className="flex items-center justify-center h-screen bg-gray-900 text-white">
    <div className="text-center">
      <div className="w-64 h-36 bg-gray-800 rounded-lg mb-4 flex items-center justify-center">
        <span className="text-gray-500">Video Player Mock</span>
      </div>
      <p>Media Player is working correctly</p>
    </div>
  </div>
);

export const Default: Story = {
  name: 'Working State',
  args: {
    children: <WorkingPlayer />,
  },
};

export const VideoError: Story = {
  name: 'Video Playback Error',
  args: {
    children: <ThrowError error="Video codec not supported" />,
  },
};

export const PlayerError: Story = {
  name: 'Player Initialization Error',
  args: {
    children: <ThrowError error="Failed to initialize player: Invalid configuration" />,
  },
};

export const NetworkError: Story = {
  name: 'Network Error',
  args: {
    children: <ThrowError error="Failed to load video: Network error" />,
  },
};

export const GenericError: Story = {
  name: 'Generic Error',
  args: {
    children: <ThrowError error="An unexpected error occurred" />,
  },
};

export const InteractiveError: Story = {
  name: 'Interactive Error Trigger',
  args: {
    children: <ErrorButton errorMessage="User triggered error" />,
  },
};

export const WithRetryCallback: Story = {
  name: 'With Retry Callback',
  args: {
    children: <ThrowError error="Error with retry functionality" />,
    onRetry: () => {
      console.log('Retry button clicked!');
      alert('Retry callback executed!');
    },
  },
};

export const LongErrorMessage: Story = {
  name: 'Long Error Message',
  args: {
    children: <ThrowError error="This is a very long error message that contains detailed information about what went wrong during the video playback process. It might include technical details about codecs, network issues, or other problems that occurred while trying to load or play the media content." />,
  },
};