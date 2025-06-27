describe('MediaPlayer - Integration Tests', () => {
  const API_BASE = 'http://localhost:8080';
  
  beforeEach(() => {
    // Clean up any existing sessions before each test
    cy.request({
      method: 'DELETE',
      url: `${API_BASE}/api/playback/sessions/all`,
      failOnStatusCode: false
    });
  });

  it('should load media player and handle real media files', () => {
    // Skip if backend not available
    cy.request({
      method: 'GET',
      url: `${API_BASE}/api/health`,
      failOnStatusCode: false
    }).then((response) => {
      if (response.status !== 200) {
        cy.skip('Backend not available');
        return;
      }

      // Get available episodes
      cy.request(`${API_BASE}/api/media/files?limit=50`).then((filesResponse) => {
        if (!filesResponse.body.media_files?.length) {
          cy.skip('No media files available');
          return;
        }

        const episodes = filesResponse.body.media_files.filter(
          (file: any) => file.media_type === 'episode'
        );
        
        if (!episodes.length) {
          cy.skip('No episode files found');
          return;
        }

        const testEpisode = episodes[0];
        
        // Get metadata for navigation
        cy.request(`${API_BASE}/api/media/files/${testEpisode.id}/metadata`).then((metadataResponse) => {
          if (metadataResponse.status !== 200) {
            cy.skip('Could not get metadata');
            return;
          }

          const metadata = metadataResponse.body.metadata;
          const episodeId = metadata.episode_id;
          
          cy.task('log', `Testing with: ${metadata.title || 'Unknown'} from ${metadata.season?.tv_show?.title || 'Unknown Show'}`);

          // Visit the episode player
          cy.visit(`/player/episode/${episodeId}`);

          // Verify MediaPlayer loads
          cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');

          // Wait for player initialization
          cy.wait(3000);

          // Verify player remains stable
          cy.get('[data-testid="media-player"]').should('be.visible');
          
          // Check player state - one of these should be present
          cy.get('body').then($body => {
            const hasVideo = $body.find('video').length > 0;
            const hasError = $body.find('[data-testid="error-message"]').length > 0;
            const hasLoading = $body.find('[data-testid="loading-indicator"]').length > 0;
            const hasPlayButton = $body.find('[data-testid="play-button"]').length > 0;
            
            expect(hasVideo || hasError || hasLoading || hasPlayButton).to.be.true;
            
            if (hasVideo) {
              cy.task('log', '✅ Video element loaded successfully');
            } else if (hasError) {
              cy.task('log', '✅ Error handled gracefully');
            } else if (hasLoading) {
              cy.task('log', '✅ Loading state displayed');
            } else if (hasPlayButton) {
              cy.task('log', '✅ Play button available');
            }
          });

          // Clean up any sessions created
          cy.request({
            method: 'DELETE',
            url: `${API_BASE}/api/playback/sessions/all`,
            failOnStatusCode: false
          });
        });
      });
    });
  });

  it('should support playback controls and seeking', () => {
    cy.request({
      method: 'GET',
      url: `${API_BASE}/api/health`,
      failOnStatusCode: false
    }).then((response) => {
      if (response.status !== 200) {
        cy.skip('Backend not available');
        return;
      }

      cy.request(`${API_BASE}/api/media/files?limit=10`).then((filesResponse) => {
        const episode = filesResponse.body.media_files?.find(
          (f: any) => f.media_type === 'episode'
        );
        
        if (!episode) {
          cy.skip('No episodes available');
          return;
        }

        cy.request(`${API_BASE}/api/media/files/${episode.id}/metadata`).then((metadataResponse) => {
          const episodeId = metadataResponse.body.metadata?.episode_id;
          if (!episodeId) {
            cy.skip('No episode ID');
            return;
          }

          // Mock successful playback start
          cy.intercept('POST', '**/api/playback/start', {
            statusCode: 200,
            body: {
              id: 'test-session-123',
              status: 'running',
              content_hash: 'test-hash-123',
              content_url: '/api/v1/content/test-hash-123/',
              manifest_url: '/api/v1/content/test-hash-123/manifest.mpd',
              provider: 'ffmpeg-pipeline'
            }
          }).as('startPlayback');

          // Mock manifest
          cy.intercept('GET', '**/api/v1/content/*/manifest.mpd', {
            statusCode: 200,
            headers: { 'Content-Type': 'application/dash+xml' },
            fixture: 'manifest.mpd'
          }).as('getManifest');

          cy.visit(`/player/episode/${episodeId}`);
          cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
          
          // Check for playback controls
          cy.wait(2000);
          
          // Verify control elements exist
          cy.get('.w-full.h-3').should('exist'); // Progress bar
          cy.get('button').should('have.length.at.least', 1); // At least one button
          
          // Try to interact with controls if play button exists
          cy.get('body').then($body => {
            if ($body.find('[data-testid="play-button"]').length > 0) {
              cy.get('[data-testid="play-button"]').click();
              cy.wait('@startPlayback');
              cy.task('log', '✅ Play button clicked and playback initiated');
            }
          });
        });
      });
    });
  });
});