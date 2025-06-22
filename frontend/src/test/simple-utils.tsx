import React from 'react';
import { render, RenderOptions } from '@testing-library/react';

// Simple render without providers for components that don't need them
export function renderSimple(
  ui: React.ReactElement,
  options?: Omit<RenderOptions, 'wrapper'>
) {
  return render(ui, options);
}

// Re-export everything from testing library
export * from '@testing-library/react';
export { renderSimple as render };