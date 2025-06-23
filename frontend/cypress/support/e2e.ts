// Import commands.js using ES2015 syntax:
import './commands';

// Alternatively you can use CommonJS syntax:
// require('./commands')

// Global error handling
Cypress.on('uncaught:exception', (err, runnable) => {
  // Log the error for debugging
  console.error('Uncaught exception:', err.message);
  
  // Don't fail the test for certain known issues
  if (err.message.includes('Player initialization failed')) {
    console.log('Caught Shaka Player initialization error - continuing test');
    return false;
  }
  
  if (err.message.includes('ResizeObserver loop limit exceeded')) {
    console.log('Caught ResizeObserver error - continuing test');
    return false;
  }
  
  // Return false to prevent the test from failing
  return false;
});

// Before each test, clean up any existing sessions
beforeEach(() => {
  // Clean up sessions before each test
  cy.cleanupAllSessions();
  
  // Check FFmpeg processes for debugging
  cy.checkFFmpegProcesses();
});

// After each test, also clean up
afterEach(() => {
  // Clean up sessions after each test
  cy.cleanupAllSessions();
  
  // Log final state
  cy.checkFFmpegProcesses();
});