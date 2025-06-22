import type { Preview } from '@storybook/react-vite';
import { Provider } from 'jotai';
import { BrowserRouter } from 'react-router-dom';
import React from 'react';
import '../src/index.css';
import '../src/styles/tokens.css';
import { ThemeProvider } from '../src/providers/ThemeProvider';

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    backgrounds: {
      default: 'dark',
      values: [
        {
          name: 'dark',
          value: '#0f172a', // slate-900
        },
        {
          name: 'light',
          value: '#f8fafc', // slate-50
        },
      ],
    },
  },
  decorators: [
    (Story) => (
      <Provider>
        <BrowserRouter>
          <ThemeProvider defaultTheme="dark">
            <div className="min-h-screen bg-background text-foreground p-4">
              <Story />
            </div>
          </ThemeProvider>
        </BrowserRouter>
      </Provider>
    ),
  ],
};

export default preview;