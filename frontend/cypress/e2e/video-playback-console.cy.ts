/// <reference types="cypress" />

describe('Video Playback Console Test', () => {
  it('should check console logs when loading episode', () => {
    // Intercept console logs
    cy.visit('/player/episode/28b93b1f-b43a-4134-abaa-c6f255eafbfd', {
      onBeforeLoad(win) {
        cy.stub(win.console, 'log').as('consoleLog');
        cy.stub(win.console, 'error').as('consoleError');
      }
    });
    
    // Wait for page to load
    cy.wait(3000);
    
    // Check if still on loading screen
    cy.get('body').then($body => {
      const bodyText = $body.text();
      cy.log(`Page content: ${bodyText.substring(0, 100)}`);
      
      if (bodyText.includes('Loading episode...')) {
        cy.log('⚠️ Still on loading screen after 3 seconds');
      } else if (bodyText.includes('Error')) {
        cy.log('❌ Error screen displayed');
      } else {
        cy.log('✅ Past loading screen');
      }
    });
    
    // Log all console.log calls
    cy.get('@consoleLog').then((stub) => {
      const calls = (stub as any).getCalls();
      cy.log(`Total console.log calls: ${calls.length}`);
      calls.forEach((call: any, index: number) => {
        cy.log(`Log ${index}: ${call.args.join(' ')}`);
      });
    });
    
    // Log all console.error calls
    cy.get('@consoleError').then((stub) => {
      const calls = (stub as any).getCalls();
      cy.log(`Total console.error calls: ${calls.length}`);
      calls.forEach((call: any, index: number) => {
        cy.log(`Error ${index}: ${call.args.join(' ')}`);
      });
    });
  });
});