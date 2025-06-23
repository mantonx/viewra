describe('Video Playback - DASH/HLS Testing', () => {
  // Test configuration
  const API_BASE = 'http://localhost:8080';
  const FRONTEND_BASE = 'http://localhost:5175';
  
  // Helper to log with timestamps
  const log = (message: string) => {
    cy.task('log', `[${new Date().toISOString().substring(11, 19)}] ${message}`);
  };

  // Setup console logging capture
  const setupConsoleCapture = () => {
    cy.window().then((win) => {
      const originalLog = win.console.log;
      const originalError = win.console.error;
      const originalWarn = win.console.warn;
      
      win.console.log = (...args) => {
        const message = args.map(arg => 
          typeof arg === 'object' ? JSON.stringify(arg, null, 2) : arg
        ).join(' ');
        
        // Log important messages
        if (message.includes('Player initialization') ||
            message.includes('Transcoding') ||
            message.includes('MediaService') ||
            message.includes('manifest') ||
            message.includes('Shaka') ||
            message.includes('HLS') ||
            message.includes('DASH') ||
            message.includes('error') ||
            message.includes('Error')) {
          log(`📋 LOG: ${message}`);
        }
        originalLog.apply(win.console, args);
      };
      
      win.console.error = (...args) => {
        const message = args.join(' ');
        log(`🔴 ERROR: ${message}`);
        originalError.apply(win.console, args);
      };
      
      win.console.warn = (...args) => {
        const message = args.join(' ');
        log(`🟡 WARN: ${message}`);
        originalWarn.apply(win.console, args);
      };
    });
  };

  beforeEach(() => {
    // Clean up any existing sessions
    cy.request({
      method: 'POST',
      url: `${API_BASE}/api/playback/stop-all`,
      failOnStatusCode: false
    });
  });

  describe('Backend API Tests', () => {
    it('should verify backend is healthy and has media files', () => {
      // Check health
      cy.request(`${API_BASE}/api/health`).then((response) => {
        expect(response.status).to.equal(200);
        log('✅ Backend is healthy');
      });

      // Get media files
      cy.request(`${API_BASE}/api/media/`).then((response) => {
        expect(response.status).to.equal(200);
        expect(response.body.media).to.be.an('array');
        expect(response.body.media.length).to.be.greaterThan(0);
        log(`✅ Found ${response.body.media.length} media files`);
        
        // Log first few files for debugging
        response.body.media.slice(0, 3).forEach((file: any) => {
          log(`  - ${file.filename} (${file.id})`);
        });
      });
    });

    it('should create and verify DASH transcoding session', () => {
      // Get a test media file
      cy.request(`${API_BASE}/api/media/`).then((response) => {
        const mediaFiles = response.body.media || [];
        const testFile = mediaFiles[0];
        
        if (!testFile) {
          throw new Error('No media files available for testing');
        }
        
        log(`🎬 Testing DASH transcoding with: ${testFile.filename}`);
        
        // Start DASH transcoding
        cy.request({
          method: 'POST',
          url: `${API_BASE}/api/playback/start`,
          body: {
            media_file_id: testFile.id,
            container: 'dash'
          }
        }).then((startResponse) => {
          expect(startResponse.status).to.equal(200);
          expect(startResponse.body.id).to.exist;
          
          const sessionId = startResponse.body.id;
          log(`✅ DASH session created: ${sessionId}`);
          
          // Verify session exists
          cy.request(`${API_BASE}/api/playback/session/${sessionId}`).then((sessionResponse) => {
            expect(sessionResponse.status).to.equal(200);
            expect(sessionResponse.body.ID).to.equal(sessionId);
            log(`✅ Session verified - Status: ${sessionResponse.body.Status}`);
          });
          
          // Check manifest availability
          cy.request({
            method: 'GET',
            url: `${API_BASE}/api/playback/stream/${sessionId}/manifest.mpd`,
            failOnStatusCode: false
          }).then((manifestResponse) => {
            log(`📄 Manifest response: ${manifestResponse.status}`);
            if (manifestResponse.status === 200) {
              log('✅ DASH manifest is available');
              expect(manifestResponse.headers['content-type']).to.include('dash+xml');
            } else {
              log('⚠️  DASH manifest not ready yet');
            }
          });
          
          // Clean up
          cy.request({
            method: 'POST',
            url: `${API_BASE}/api/playback/stop`,
            body: { session_id: sessionId }
          });
        });
      });
    });

    it('should create and verify HLS transcoding session', () => {
      // Get a test media file
      cy.request(`${API_BASE}/api/media/`).then((response) => {
        const mediaFiles = response.body.media || [];
        const testFile = mediaFiles[0];
        
        if (!testFile) {
          throw new Error('No media files available for testing');
        }
        
        log(`🎬 Testing HLS transcoding with: ${testFile.filename}`);
        
        // Start HLS transcoding
        cy.request({
          method: 'POST',
          url: `${API_BASE}/api/playback/start`,
          body: {
            media_file_id: testFile.id,
            container: 'hls'
          }
        }).then((startResponse) => {
          expect(startResponse.status).to.equal(200);
          expect(startResponse.body.id).to.exist;
          
          const sessionId = startResponse.body.id;
          log(`✅ HLS session created: ${sessionId}`);
          
          // Verify session exists
          cy.request(`${API_BASE}/api/playback/session/${sessionId}`).then((sessionResponse) => {
            expect(sessionResponse.status).to.equal(200);
            expect(sessionResponse.body.ID).to.equal(sessionId);
            log(`✅ Session verified - Status: ${sessionResponse.body.Status}`);
          });
          
          // Check playlist availability
          cy.request({
            method: 'GET',
            url: `${API_BASE}/api/playback/stream/${sessionId}/playlist.m3u8`,
            failOnStatusCode: false
          }).then((playlistResponse) => {
            log(`📄 Playlist response: ${playlistResponse.status}`);
            if (playlistResponse.status === 200) {
              log('✅ HLS playlist is available');
              expect(playlistResponse.headers['content-type']).to.include('m3u8');
            } else {
              log('⚠️  HLS playlist not ready yet');
            }
          });
          
          // Clean up
          cy.request({
            method: 'POST',
            url: `${API_BASE}/api/playback/stop`,
            body: { session_id: sessionId }
          });
        });
      });
    });
  });

  describe('Frontend Player Tests', () => {
    it('should test DASH playback in the player', () => {
      // Get a test episode
      cy.request(`${API_BASE}/api/media/`).then((response) => {
        const mediaFiles = response.body.media || [];
        const episodes = mediaFiles.filter((f: any) => f.type === 'episode');
        
        if (episodes.length === 0) {
          log('⚠️  No episodes found, using first media file');
        }
        
        const testFile = episodes[0] || mediaFiles[0];
        if (!testFile) {
          throw new Error('No media files available');
        }
        
        log(`🎬 Testing DASH player with: ${testFile.filename}`);
        
        // Get metadata to find episode ID
        cy.request(`${API_BASE}/api/media/${testFile.id}/metadata`).then((metaResponse) => {
          const episodeId = metaResponse.body.episode_id || testFile.id;
          
          // Set up console capture
          setupConsoleCapture();
          
          // Intercept playback start to ensure DASH
          cy.intercept('POST', '**/api/playback/start', (req) => {
            log(`🔄 Intercepting playback start - forcing DASH`);
            req.body.container = 'dash';
            req.continue();
          }).as('dashStart');
          
          // Visit player
          cy.visit(`${FRONTEND_BASE}/player/episode/${episodeId}`);
          
          // Wait for player to load
          cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
          log('✅ Media player loaded');
          
          // Wait for initialization
          cy.wait(5000);
          
          // Check player state
          cy.get('body').then(($body) => {
            const bodyText = $body.text();
            
            if (bodyText.includes('Episode not found')) {
              log('❌ Episode not found');
            } else if (bodyText.includes('Player initialization failed')) {
              log('⚠️  Player initialization failed - checking for Shaka error');
              
              // Look for video element
              if ($body.find('video').length > 0) {
                cy.get('video').then(($video) => {
                  const video = $video[0] as HTMLVideoElement;
                  log(`📺 Video element state: readyState=${video.readyState}, networkState=${video.networkState}`);
                });
              }
            } else if ($body.find('video').length > 0) {
              cy.get('video').then(($video) => {
                const video = $video[0] as HTMLVideoElement;
                log(`✅ Video element found - src: ${video.src || 'no src'}`);
                log(`📺 Video state: readyState=${video.readyState}, networkState=${video.networkState}`);
              });
            }
          });
          
          // Take screenshot
          cy.screenshot('dash-playback-test');
        });
      });
    });

    it('should test HLS playback in the player', () => {
      // Get a test episode
      cy.request(`${API_BASE}/api/media/`).then((response) => {
        const mediaFiles = response.body.media || [];
        const episodes = mediaFiles.filter((f: any) => f.type === 'episode');
        
        const testFile = episodes[0] || mediaFiles[0];
        if (!testFile) {
          throw new Error('No media files available');
        }
        
        log(`🎬 Testing HLS player with: ${testFile.filename}`);
        
        // Get metadata to find episode ID
        cy.request(`${API_BASE}/api/media/${testFile.id}/metadata`).then((metaResponse) => {
          const episodeId = metaResponse.body.episode_id || testFile.id;
          
          // Set up console capture
          setupConsoleCapture();
          
          // Intercept playback start to ensure HLS
          cy.intercept('POST', '**/api/playback/start', (req) => {
            log(`🔄 Intercepting playback start - forcing HLS`);
            req.body.container = 'hls';
            req.continue();
          }).as('hlsStart');
          
          // Visit player
          cy.visit(`${FRONTEND_BASE}/player/episode/${episodeId}`);
          
          // Wait for player to load
          cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
          log('✅ Media player loaded');
          
          // Wait for initialization
          cy.wait(5000);
          
          // Check player state
          cy.get('body').then(($body) => {
            const bodyText = $body.text();
            
            if (bodyText.includes('Episode not found')) {
              log('❌ Episode not found');
            } else if (bodyText.includes('Player initialization failed')) {
              log('⚠️  Player initialization failed - checking for HLS.js error');
            } else if ($body.find('video').length > 0) {
              cy.get('video').then(($video) => {
                const video = $video[0] as HTMLVideoElement;
                log(`✅ Video element found - src: ${video.src || 'no src'}`);
                log(`📺 Video state: readyState=${video.readyState}, networkState=${video.networkState}`);
              });
            }
          });
          
          // Take screenshot
          cy.screenshot('hls-playback-test');
        });
      });
    });
  });

  describe('Network Monitoring Tests', () => {
    it('should monitor all network requests during DASH playback', () => {
      const networkLogs: string[] = [];
      
      // Intercept all playback-related requests
      cy.intercept('**', (req) => {
        if (req.url.includes('playback') || 
            req.url.includes('manifest') || 
            req.url.includes('stream') ||
            req.url.includes('.mpd') ||
            req.url.includes('.m4s')) {
          networkLogs.push(`${req.method} ${req.url}`);
        }
      });
      
      // Get test file and play
      cy.request(`${API_BASE}/api/media/`).then((response) => {
        const testFile = response.body.media[0];
        if (!testFile) return;
        
        cy.request(`${API_BASE}/api/media/${testFile.id}/metadata`).then((metaResponse) => {
          const episodeId = metaResponse.body.episode_id || testFile.id;
          
          cy.visit(`${FRONTEND_BASE}/player/episode/${episodeId}`);
          cy.wait(10000);
          
          // Log all captured network requests
          log('📡 Network requests captured:');
          networkLogs.forEach(req => log(`  - ${req}`));
        });
      });
    });
  });

  describe('Transcoding Debug Tests', () => {
    it('should verify FFmpeg is producing valid DASH output', () => {
      cy.request(`${API_BASE}/api/media/`).then((response) => {
        const testFile = response.body.media[0];
        if (!testFile) return;
        
        // Start DASH transcoding
        cy.request({
          method: 'POST',
          url: `${API_BASE}/api/playback/start`,
          body: {
            media_file_id: testFile.id,
            container: 'dash'
          }
        }).then((startResponse) => {
          const sessionId = startResponse.body.id;
          log(`🎬 Started DASH session: ${sessionId}`);
          
          // Wait for transcoding to produce some output
          cy.wait(5000);
          
          // Check session status
          cy.request(`${API_BASE}/api/playback/session/${sessionId}`).then((statusResponse) => {
            log(`📊 Session status: ${statusResponse.body.Status}`);
            log(`📊 Progress: ${statusResponse.body.Progress || 'N/A'}`);
            
            if (statusResponse.body.Error) {
              log(`❌ Transcoding error: ${statusResponse.body.Error}`);
            }
          });
          
          // Try to get manifest
          cy.request({
            method: 'GET',
            url: `${API_BASE}/api/playback/stream/${sessionId}/manifest.mpd`,
            failOnStatusCode: false
          }).then((manifestResponse) => {
            if (manifestResponse.status === 200) {
              log('✅ DASH manifest exists');
              log(`📄 Manifest size: ${manifestResponse.body.length} bytes`);
              
              // Check if it's valid XML
              if (manifestResponse.body.includes('<?xml') && manifestResponse.body.includes('<MPD')) {
                log('✅ Manifest appears to be valid DASH MPD');
              } else {
                log('❌ Manifest does not appear to be valid DASH MPD');
              }
            } else {
              log(`❌ Failed to get manifest: ${manifestResponse.status}`);
            }
          });
          
          // Clean up
          cy.request({
            method: 'POST',
            url: `${API_BASE}/api/playback/stop`,
            body: { session_id: sessionId }
          });
        });
      });
    });
  });
});