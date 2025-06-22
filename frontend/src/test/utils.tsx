import React from 'react';
import { render, RenderOptions } from '@testing-library/react';
import { Provider } from 'jotai';
import { MemoryRouter } from 'react-router-dom';

// Custom render function that includes providers
interface CustomRenderOptions extends Omit<RenderOptions, 'wrapper'> {
  initialRoutes?: string[];
}

export function renderWithProviders(
  ui: React.ReactElement,
  {
    initialRoutes = ['/'],
    ...renderOptions
  }: CustomRenderOptions = {}
) {
  function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <Provider>
        <MemoryRouter initialEntries={initialRoutes}>
          {children}
        </MemoryRouter>
      </Provider>
    );
  }

  return {
    ...render(ui, { wrapper: Wrapper, ...renderOptions }),
  };
}

// Re-export everything from testing library
export * from '@testing-library/react';
export { renderWithProviders as render };