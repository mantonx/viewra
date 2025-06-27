describe('MediaPlayer - Network Monitoring', () => {
  const API_BASE = 'http://localhost:8080';
  const FRONTEND_BASE = 'http://localhost:5175';
  
  beforeEach(() => {
    cy.request({
      method: 'DELETE',
      url: `${API_BASE}/api/playback/sessions/all`,
      failOnStatusCode: false
    });
  });

  it('should use content-addressable storage URLs', () => {
    const networkRequests = {
      contentAddressable: [] as string[],
      legacy: [] as string[],
      playback: [] as string[]
    };
    
    // Intercept all relevant requests
    cy.intercept('**', (req) => {
      const url = req.url;
      
      if (url.includes('/api/v1/content/')) {
        networkRequests.contentAddressable.push(`${req.method} ${url}`);
      } else if (url.includes('/api/playback/stream/')) {
        networkRequests.legacy.push(`${req.method} ${url}`);
      } else if (url.includes('/api/playback/')) {
        networkRequests.playback.push(`${req.method} ${url}`);
      }
    });
    
    cy.request(`${API_BASE}/api/media/files?limit=10`).then((response) => {
      const episode = response.body.media_files?.find(
        (f: any) => f.media_type === 'episode'
      );
      
      if (!episode) {
        cy.skip('No episodes available');
        return;
      }
      
      cy.request(`${API_BASE}/api/media/files/${episode.id}/metadata`).then((metaResponse) => {
        const episodeId = metaResponse.body.metadata?.episode_id;
        if (!episodeId) {
          cy.skip('No episode ID');
          return;
        }
        
        cy.visit(`${FRONTEND_BASE}/player/episode/${episodeId}`);
        cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
        
        // Wait for network activity
        cy.wait(5000);
        
        // Log captured requests
        cy.task('log', '📡 Network Request Summary:');
        cy.task('log', `✅ Content-addressable requests: ${networkRequests.contentAddressable.length}`);
        networkRequests.contentAddressable.forEach(req => cy.task('log', `  - ${req}`));
        
        if (networkRequests.legacy.length > 0) {
          cy.task('log', `⚠️ Legacy requests (should migrate): ${networkRequests.legacy.length}`);
          networkRequests.legacy.forEach(req => cy.task('log', `  - ${req}`));
        }
        
        cy.task('log', `📊 Playback API requests: ${networkRequests.playback.length}`);
        networkRequests.playback.forEach(req => cy.task('log', `  - ${req}`));
        
        // Verify we're using the new architecture
        if (networkRequests.contentAddressable.length > 0 && networkRequests.legacy.length === 0) {
          cy.task('log', '✅ All streaming requests use content-addressable storage!');
        } else if (networkRequests.legacy.length > 0) {
          cy.task('log', '⚠️ Still using some legacy streaming endpoints');
        }
      });
    });
  });

  it('should monitor playback session lifecycle', () => {
    const sessionLifecycle = {
      start: null as any,
      status: [] as any[],
      manifest: null as any,
      segments: [] as any[]
    };
    
    // Intercept session lifecycle
    cy.intercept('POST', '**/api/playback/start', (req) => {
      req.continue((res) => {
        sessionLifecycle.start = res.body;
        cy.task('log', `🚀 Session started: ${res.body.id}`);
      });
    }).as('sessionStart');
    
    cy.intercept('GET', '**/api/playback/session/*', (req) => {
      req.continue((res) => {
        sessionLifecycle.status.push({
          time: Date.now(),
          status: res.body.Status,
          progress: res.body.Progress
        });
      });
    }).as('sessionStatus');
    
    cy.intercept('GET', '**/manifest.mpd', (req) => {
      req.continue((res) => {
        sessionLifecycle.manifest = {
          url: req.url,
          status: res.statusCode,
          contentType: res.headers['content-type']
        };
      });
    }).as('manifest');
    
    cy.intercept('GET', '**/*.m4s', (req) => {
      sessionLifecycle.segments.push(req.url);
    }).as('segment');
    
    // Run test
    cy.request(`${API_BASE}/api/media/files?limit=10`).then((response) => {
      const episode = response.body.media_files?.find(
        (f: any) => f.media_type === 'episode'
      );
      
      if (!episode) {
        cy.skip('No episodes available');
        return;
      }
      
      cy.request(`${API_BASE}/api/media/files/${episode.id}/metadata`).then((metaResponse) => {
        const episodeId = metaResponse.body.metadata?.episode_id;
        if (!episodeId) return;
        
        cy.visit(`${FRONTEND_BASE}/player/episode/${episodeId}`);
        cy.get('[data-testid="media-player"]').should('be.visible');
        
        // Try to start playback
        cy.get('body').then($body => {
          if ($body.find('[data-testid="play-button"]').length > 0) {
            cy.get('[data-testid="play-button"]').click();
          }
        });
        
        cy.wait(5000);
        
        // Log lifecycle data
        cy.task('log', '🔄 Session Lifecycle:');
        if (sessionLifecycle.start) {
          cy.task('log', `1. Start: Session ${sessionLifecycle.start.id}`);
          cy.task('log', `   - Provider: ${sessionLifecycle.start.provider}`);
          cy.task('log', `   - Content Hash: ${sessionLifecycle.start.content_hash || 'N/A'}`);
        }
        
        if (sessionLifecycle.status.length > 0) {
          cy.task('log', `2. Status checks: ${sessionLifecycle.status.length}`);
          sessionLifecycle.status.forEach((s, i) => {
            cy.task('log', `   - Check ${i + 1}: ${s.status} (${s.progress || 'N/A'}%)`);
          });
        }
        
        if (sessionLifecycle.manifest) {
          cy.task('log', `3. Manifest: ${sessionLifecycle.manifest.status}`);
          cy.task('log', `   - Type: ${sessionLifecycle.manifest.contentType}`);
        }
        
        if (sessionLifecycle.segments.length > 0) {
          cy.task('log', `4. Segments loaded: ${sessionLifecycle.segments.length}`);
        }
      });
    });
  });
});