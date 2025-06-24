import type { Meta, StoryObj } from '@storybook/react-vite';
import { QualityIndicator } from './QualityIndicator';

// Note: This component requires Vidstack context to work properly
// For demonstration purposes, we'll show the component structure

const meta = {
  title: 'Components/MediaPlayer/QualityIndicator',
  component: QualityIndicator,
  parameters: {
    layout: 'fullscreen',
    backgrounds: {
      default: 'dark',
    },
    docs: {
      description: {
        component: 'QualityIndicator displays the current video quality and bitrate. It requires Vidstack MediaStore context to function properly.',
      },
    },
  },
  decorators: [
    (Story) => (
      <div className="relative h-screen w-full bg-gray-900">
        <div className="flex items-center justify-center h-full text-white">
          <div className="text-center p-8 bg-gray-800 rounded-lg">
            <p className="mb-4">QualityIndicator requires Vidstack context.</p>
            <p className="text-sm text-gray-400">
              In a real implementation, this would show:
              <br />• Quality label (4K, 1080p, 720p, etc.)
              <br />• Current bitrate in Mbps
              <br />• Color-coded border based on quality
              <br />• Auto-hide after 3 seconds
            </p>
          </div>
        </div>
        {/* The actual component would be rendered here with Vidstack context */}
        {/* <Story /> */}
      </div>
    ),
  ],
} satisfies Meta<typeof QualityIndicator>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  name: 'Component Info',
};

// Example of what the component looks like at different quality levels
export const QualityExamples: Story = {
  name: 'Quality Level Examples',
  render: () => (
    <div className="grid grid-cols-2 gap-8 p-8">
      <div className="space-y-4">
        <h3 className="text-white font-semibold">Quality Levels:</h3>
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: '#4ade80' }}></div>
            <span className="text-white">4K / 1080p - Excellent</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: '#facc15' }}></div>
            <span className="text-white">720p - Good</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: '#fb923c' }}></div>
            <span className="text-white">480p - Fair</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: '#f87171' }}></div>
            <span className="text-white">360p / 240p - Low</span>
          </div>
        </div>
      </div>
      
      <div className="space-y-4">
        <h3 className="text-white font-semibold">Features:</h3>
        <ul className="text-white/80 text-sm space-y-1">
          <li>• Auto-detects quality changes</li>
          <li>• Shows upgrade animation on quality improvement</li>
          <li>• Displays current bitrate</li>
          <li>• Auto-hides after 3 seconds</li>
          <li>• Color-coded quality indicator</li>
        </ul>
      </div>
    </div>
  ),
  parameters: {
    layout: 'centered',
  },
};