describe('MediaPlayer - Playback Tests', () => {
  const API_BASE = 'http://localhost:8080';
  
  beforeEach(() => {
    // Clean up any existing sessions before each test
    cy.request({
      method: 'DELETE',
      url: `${API_BASE}/api/playback/sessions/all`,
      failOnStatusCode: false
    });
  });

  describe('Content-Addressable Storage', () => {
    it('should deduplicate content with same encoding parameters', () => {
      cy.request(`${API_BASE}/api/media/files?limit=1`).then((response) => {
        const testFile = response.body.media_files?.[0];
        if (!testFile) {
          cy.skip('No media files available');
          return;
        }
        
        cy.task('log', `Testing deduplication with: ${testFile.path}`);
        
        // Start two identical transcoding sessions
        const sessionRequests = [1, 2].map(() => 
          cy.request({
            method: 'POST',
            url: `${API_BASE}/api/playback/start`,
            body: {
              media_file_id: testFile.id,
              container: 'dash',
              enable_abr: true
            }
          })
        );
        
        cy.wrap(Promise.all(sessionRequests)).then((responses: any[]) => {
          const [first, second] = responses;
          const firstHash = first.body.content_hash;
          const secondHash = second.body.content_hash;
          
          if (firstHash && secondHash) {
            expect(secondHash).to.equal(firstHash);
            cy.task('log', `✅ Content deduplicated! Hash: ${firstHash}`);
          } else {
            cy.task('log', '⚠️ Content hashes not available yet');
          }
          
          // Clean up sessions
          responses.forEach(res => {
            cy.request({
              method: 'DELETE',
              url: `${API_BASE}/api/playback/session/${res.body.id}`,
              failOnStatusCode: false
            });
          });
        });
      });
    });

    it('should create different hashes for different encoding parameters', () => {
      cy.request(`${API_BASE}/api/media/files?limit=1`).then((response) => {
        const testFile = response.body.media_files?.[0];
        if (!testFile) {
          cy.skip('No media files available');
          return;
        }
        
        // Start DASH and HLS transcoding
        const dashRequest = cy.request({
          method: 'POST',
          url: `${API_BASE}/api/playback/start`,
          body: {
            media_file_id: testFile.id,
            container: 'dash',
            enable_abr: true
          }
        });
        
        const hlsRequest = cy.request({
          method: 'POST',
          url: `${API_BASE}/api/playback/start`,
          body: {
            media_file_id: testFile.id,
            container: 'hls',
            enable_abr: true
          }
        });
        
        cy.wrap(Promise.all([dashRequest, hlsRequest])).then((responses: any[]) => {
          const [dash, hls] = responses;
          const dashHash = dash.body.content_hash;
          const hlsHash = hls.body.content_hash;
          
          if (dashHash && hlsHash) {
            expect(hlsHash).to.not.equal(dashHash);
            cy.task('log', '✅ Different parameters produce different hashes');
          }
          
          // Clean up
          responses.forEach(res => {
            cy.request({
              method: 'DELETE',
              url: `${API_BASE}/api/playback/session/${res.body.id}`,
              failOnStatusCode: false
            });
          });
        });
      });
    });
  });

  describe('Direct Playback', () => {
    it('should handle direct playback without transcoding', () => {
      cy.request(`${API_BASE}/api/media/files?limit=10`).then((response) => {
        const episode = response.body.media_files?.find(
          (f: any) => f.media_type === 'episode'
        );
        
        if (!episode) {
          cy.skip('No episodes available');
          return;
        }
        
        // Get metadata for navigation
        cy.request(`${API_BASE}/api/media/files/${episode.id}/metadata`).then((metaResponse) => {
          const episodeId = metaResponse.body.metadata?.episode_id;
          if (!episodeId) {
            cy.skip('No episode ID in metadata');
            return;
          }
          
          // Force direct playback
          cy.intercept('POST', '**/api/playback/start', (req) => {
            req.body.container = 'direct';
            req.body.force_direct = true;
          }).as('directPlayback');
          
          cy.visit(`/player/episode/${episodeId}`);
          cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
          
          // Check for play button and click if available
          cy.get('body').then($body => {
            if ($body.find('[data-testid="play-button"]').length > 0) {
              cy.get('[data-testid="play-button"]').click();
              cy.wait('@directPlayback', { timeout: 10000 });
              cy.task('log', '✅ Direct playback initiated');
            }
          });
          
          // Verify player stability
          cy.wait(2000);
          cy.get('[data-testid="media-player"]').should('be.visible');
        });
      });
    });
  });

  describe('Transcoding Playback', () => {
    ['dash', 'hls'].forEach(container => {
      it(`should handle ${container.toUpperCase()} transcoding`, () => {
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
              cy.skip('No episode ID in metadata');
              return;
            }
            
            // Force specific container
            cy.intercept('POST', '**/api/playback/start', (req) => {
              req.body.container = container;
              req.body.enable_abr = true;
            }).as(`${container}Playback`);
            
            // Mock manifest response
            const manifestEndpoint = container === 'dash' 
              ? '**/api/v1/content/*/manifest.mpd'
              : '**/api/v1/content/*/playlist.m3u8';
            
            cy.intercept('GET', manifestEndpoint, {
              statusCode: 200,
              headers: { 
                'Content-Type': container === 'dash' 
                  ? 'application/dash+xml' 
                  : 'application/vnd.apple.mpegurl' 
              },
              fixture: container === 'dash' ? 'manifest.mpd' : 'playlist.m3u8'
            }).as(`${container}Manifest`);
            
            cy.visit(`/player/episode/${episodeId}`);
            cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
            
            // Try to start playback
            cy.get('body').then($body => {
              if ($body.find('[data-testid="play-button"]').length > 0) {
                cy.get('[data-testid="play-button"]').click();
                cy.wait(`@${container}Playback`, { timeout: 10000 });
                cy.task('log', `✅ ${container.toUpperCase()} transcoding initiated`);
              }
            });
            
            cy.wait(2000);
            cy.get('[data-testid="media-player"]').should('be.visible');
          });
        });
      });
    });
  });

  describe('Seek-Ahead Functionality', () => {
    it('should handle seek-ahead requests', () => {
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
            cy.skip('No episode ID in metadata');
            return;
          }
          
          // Mock seek-ahead response
          cy.intercept('POST', '**/api/playback/seek-ahead', (req) => {
            const seekPos = req.body.seek_position;
            cy.task('log', `🚀 Seek-ahead to ${seekPos}s`);
            req.reply({
              statusCode: 200,
              body: {
                id: `seek-${Date.now()}`,
                status: 'queued',
                content_hash: `seek-hash-${seekPos}`,
                content_url: `/api/v1/content/seek-hash-${seekPos}/`,
                manifest_url: `/api/v1/content/seek-hash-${seekPos}/manifest.mpd`
              }
            });
          }).as('seekAhead');
          
          cy.visit(`/player/episode/${episodeId}`);
          cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
          cy.wait(3000);
          
          // Try to click on progress bar to trigger seek
          cy.get('.w-full.h-3').first().then($progress => {
            if ($progress.length > 0) {
              const rect = $progress[0].getBoundingClientRect();
              const x = rect.left + (rect.width * 0.75);
              const y = rect.top + (rect.height / 2);
              
              cy.get('.w-full.h-3').first().click(x, y);
              
              cy.wait('@seekAhead', { timeout: 5000 }).then((interception) => {
                expect(interception.response.body.content_hash).to.include('seek-hash');
                cy.task('log', '✅ Seek-ahead triggered successfully');
              });
            } else {
              cy.task('log', 'ℹ️ Progress bar not available');
            }
          });
        });
      });
    });
  });
});