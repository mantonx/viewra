import type { Meta, StoryObj } from '@storybook/react-vite';
import { MediaInfoOverlay } from './MediaInfoOverlay';
import type { Episode, Movie } from '../../types';

const meta = {
  title: 'MediaPlayer/MediaInfoOverlay',
  component: MediaInfoOverlay,
  parameters: {
    layout: 'fullscreen',
  },
  tags: ['autodocs'],
  argTypes: {
    position: {
      control: 'select',
      options: ['top-left', 'top-right', 'bottom-left', 'bottom-right'],
      description: 'Position of the overlay',
    },
    showOnHover: {
      control: 'boolean',
      description: 'Only show overlay on hover',
    },
    autoHide: {
      control: 'boolean',
      description: 'Automatically hide after delay',
    },
    autoHideDelay: {
      control: { type: 'range', min: 1000, max: 10000, step: 1000 },
      description: 'Auto hide delay in milliseconds',
    },
  },
  decorators: [
    (Story) => (
      <div className="relative w-full h-[600px] bg-slate-900 bg-[url('https://via.placeholder.com/1920x1080/1e293b/64748b?text=Video+Player')] bg-cover bg-center">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof MediaInfoOverlay>;

export default meta;
type Story = StoryObj<typeof meta>;

const mockEpisode: Episode = {
  id: 'ep-123',
  type: 'episode',
  title: 'The Beginning',
  episode_number: 1,
  season_number: 1,
  description: 'In the series premiere, our heroes embark on an epic journey that will change their lives forever.',
  duration: 3240,
  air_date: '2024-01-15',
  still_image: 'https://via.placeholder.com/1920x1080',
  series: {
    id: 'series-456',
    title: 'Epic Adventure',
    description: 'An amazing series about adventure and discovery',
    poster: 'https://via.placeholder.com/300x450',
    backdrop: 'https://via.placeholder.com/1920x1080',
  },
};

const mockMovie: Movie = {
  id: 'movie-789',
  type: 'movie',
  title: 'The Great Adventure',
  description: 'An epic tale of courage, friendship, and discovery in a world of wonder.',
  duration: 7200,
  release_date: '2023-12-25',
  runtime: 120,
  poster: 'https://via.placeholder.com/300x450',
  backdrop: 'https://via.placeholder.com/1920x1080',
};

export const EpisodeTopLeft: Story = {
  args: {
    media: mockEpisode,
    position: 'top-left',
    showOnHover: false,
    autoHide: false,
  },
};

export const EpisodeTopRight: Story = {
  args: {
    media: mockEpisode,
    position: 'top-right',
    showOnHover: false,
    autoHide: false,
  },
};

export const MovieBottomLeft: Story = {
  args: {
    media: mockMovie,
    position: 'bottom-left',
    showOnHover: false,
    autoHide: false,
  },
};

export const MovieBottomRight: Story = {
  args: {
    media: mockMovie,
    position: 'bottom-right',
    showOnHover: false,
    autoHide: false,
  },
};

export const WithAutoHide: Story = {
  args: {
    media: mockEpisode,
    position: 'top-left',
    showOnHover: false,
    autoHide: true,
    autoHideDelay: 3000,
  },
};

export const ShowOnHover: Story = {
  args: {
    media: mockMovie,
    position: 'top-left',
    showOnHover: true,
    autoHide: false,
  },
};

export const LongTitle: Story = {
  args: {
    media: {
      ...mockEpisode,
      title: 'This Is a Very Long Episode Title That Might Need to Be Truncated',
      series: {
        ...mockEpisode.series,
        title: 'An Extremely Long Series Title That Goes On and On',
      },
    },
    position: 'top-left',
    showOnHover: false,
    autoHide: false,
  },
};

export const NoMedia: Story = {
  args: {
    media: null,
    position: 'top-left',
    showOnHover: false,
    autoHide: false,
  },
};

export const MinimalInfo: Story = {
  args: {
    media: {
      id: 'min-1',
      type: 'movie',
      title: 'Minimal Movie',
    } as Movie,
    position: 'top-left',
    showOnHover: false,
    autoHide: false,
  },
};

export const WithCustomStyling: Story = {
  args: {
    media: mockEpisode,
    position: 'top-left',
    showOnHover: false,
    autoHide: false,
    className: 'bg-gradient-to-r from-purple-900/90 to-blue-900/90 backdrop-blur-xl',
  },
};