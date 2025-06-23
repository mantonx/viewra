/// <reference types="cypress" />

describe('Video Playback UI Test', () => {
  beforeEach(() => {
    // Clean up any existing sessions
    cy.request({
      method: 'DELETE',
      url: 'http://localhost:8080/api/playback/sessions/all',
      failOnStatusCode: false,
    });
  });

  it('should play video in episode player', () => {
    // Get an episode
    cy.request({
      method: 'GET',
      url: 'http://localhost:8080/api/media/files?limit=100',
    }).then((response) => {
      const episode = response.body.media_files.find(f => 
        f.media_type === 'episode' && f.video_codec && f.media_id
      );
      
      expect(episode).to.exist;
      cy.log(`Found episode: ${episode.path}, media_id: ${episode.media_id}`);
      
      // Navigate to episode player
      cy.visit(`/player/episode/${episode.media_id}`);
      
      // Check if we're stuck on loading
      cy.get('body').then($body => {
        if ($body.text().includes('Loading episode...')) {
          cy.log('⚠️ Stuck on loading screen');
          
          // Check console for errors
          cy.window().then((win) => {
            cy.log('Console errors:', win.console.error);
          });
        }
      });
      
      // Wait for player to load
      cy.get('[data-testid="media-player"]', { timeout: 10000 }).should('exist');
      
      // Wait for manifest to be ready
      cy.wait(5000);
      
      // Check if video element exists
      cy.get('video').should('exist').then(($video) => {
        const video = $video[0] as HTMLVideoElement;
        
        // Log video state
        cy.log(`Video ready state: ${video.readyState}`);
        cy.log(`Video src: ${video.src || 'No src'}`);
        cy.log(`Video duration: ${video.duration}`);
        cy.log(`Video error: ${video.error?.message || 'No error'}`);
        
        // Check if Shaka Player is loaded
        cy.window().then((win) => {
          if (win.shaka) {
            cy.log('Shaka Player is loaded');
          } else {
            cy.log('Shaka Player NOT loaded');
          }
        });
      });
      
      // Check for any error messages
      cy.get('[data-testid="error-message"]', { timeout: 1000 }).should('not.exist');
      
      // Check loading indicator
      cy.get('[data-testid="loading-indicator"]', { timeout: 1000 }).then(($loading) => {
        if ($loading.length > 0) {
          cy.log(`Loading state: ${$loading.text()}`);
        } else {
          cy.log('No loading indicator visible');
        }
      });
      
      // Check if play button exists
      cy.get('[data-testid="play-button"]', { timeout: 2000 }).should('exist');
    });
  });
});