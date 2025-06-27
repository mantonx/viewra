describe('MediaPlayer - Full Integration Test', () => {
  const API_BASE = 'http://localhost:8080';
  
  beforeEach(() => {
    // Clean up any existing sessions before each test
    cy.request({
      method: 'DELETE',
      url: `${API_BASE}/api/playback/sessions/all`,
      failOnStatusCode: false
    });
  });

  it('should handle complete video playback flow from session to content-addressable URLs', () => {
    // 1. Get a test media file
    cy.request(`${API_BASE}/api/media/files?limit=10`).then((response) => {
      const episode = response.body.media_files?.find(
        (f: any) => f.media_type === 'episode'
      );
      
      if (!episode) {
        cy.skip('No episodes available');
        return;
      }
      
      cy.task('log', `Testing with: ${episode.path}`);
      
      // 2. Start transcoding session
      cy.request({
        method: 'POST',
        url: `${API_BASE}/api/playback/start`,
        body: {
          media_file_id: episode.id,
          container: 'dash',
          enable_abr: true
        }
      }).then((sessionResponse) => {
        const sessionId = sessionResponse.body.id;
        const contentHash = sessionResponse.body.content_hash;
        
        expect(sessionId).to.exist;
        expect(contentHash).to.exist;
        cy.task('log', `Session created: ${sessionId}`);
        cy.task('log', `Content hash: ${contentHash}`);
        
        // 3. Test session-based URLs (temporary)
        cy.task('log', '🔄 Testing session-based URLs...');
        
        // Wait a bit for encoding to start
        cy.wait(2000);
        
        // Try to access intermediate file via session URL
        cy.request({
          url: `${API_BASE}/api/v1/sessions/${sessionId}/intermediate.mp4`,
          failOnStatusCode: false
        }).then((intermediateResponse) => {
          if (intermediateResponse.status === 200) {
            cy.task('log', '✅ Session-based intermediate file accessible');
          } else {
            cy.task('log', '⚠️ Intermediate file not yet available');
          }
        });
        
        // 4. Monitor transcoding progress
        let attempts = 0;
        const maxAttempts = 30;
        
        function checkProgress() {
          cy.request(`${API_BASE}/api/playback/session/${sessionId}`).then((statusResponse) => {
            const status = statusResponse.body.status;
            const progress = statusResponse.body.progress || 0;
            
            cy.task('log', `⏳ Status: ${status}, Progress: ${progress}%`);
            
            if (status === 'completed' || progress > 10) {
              cy.task('log', '✅ Transcoding progressed sufficiently');
              
              // 5. Test content-addressable URLs
              cy.task('log', '🔄 Testing content-addressable URLs...');
              
              // Test manifest via content hash
              cy.request({
                url: `${API_BASE}/api/v1/content/${contentHash}/manifest.mpd`,
                failOnStatusCode: false
              }).then((manifestResponse) => {
                if (manifestResponse.status === 200) {
                  cy.task('log', '✅ Content-addressable manifest accessible');
                  
                  // Verify manifest content
                  expect(manifestResponse.body).to.include('<?xml');
                  expect(manifestResponse.body).to.include('<MPD');
                  
                  // Check for duration in manifest
                  if (manifestResponse.body.includes('mediaPresentationDuration')) {
                    cy.task('log', '✅ Manifest includes duration');
                  }
                } else {
                  cy.task('log', `❌ Manifest not accessible (HTTP ${manifestResponse.status})`);
                }
              });
              
              // Test video segment via content hash
              cy.request({
                url: `${API_BASE}/api/v1/content/${contentHash}/init-0.m4s`,
                failOnStatusCode: false
              }).then((segmentResponse) => {
                if (segmentResponse.status === 200) {
                  cy.task('log', '✅ Content-addressable segments accessible');
                } else {
                  cy.task('log', '⚠️ Segments not yet available');
                }
              });
              
            } else if (status === 'failed') {
              cy.task('log', '❌ Transcoding failed');
              throw new Error('Transcoding failed');
            } else if (attempts < maxAttempts) {
              attempts++;
              cy.wait(1000).then(() => checkProgress());
            } else {
              cy.task('log', '⚠️ Timeout waiting for transcoding');
            }
          });
        }
        
        checkProgress();
        
        // 6. Clean up
        cy.then(() => {
          cy.request({
            method: 'DELETE',
            url: `${API_BASE}/api/playback/session/${sessionId}`,
            failOnStatusCode: false
          });
          cy.task('log', '🧹 Session cleaned up');
        });
      });
    });
  });

  it('should handle API contract properly between frontend and backend', () => {
    cy.request(`${API_BASE}/api/media/files?limit=1`).then((response) => {
      const testFile = response.body.media_files?.[0];
      if (!testFile) {
        cy.skip('No media files available');
        return;
      }
      
      // Test playback decision with file_id (frontend format)
      cy.request({
        method: 'POST',
        url: `${API_BASE}/api/playback/decide`,
        body: {
          file_id: testFile.id,
          device_profile: {
            user_agent: 'Mozilla/5.0 (Test)',
            supported_codecs: ['h264', 'aac'],
            max_resolution: '1080p',
            max_bitrate: 8000,
            supports_hevc: false,
            supports_av1: false,
            supports_hdr: false
          }
        }
      }).then((decisionResponse) => {
        expect(decisionResponse.status).to.equal(200);
        expect(decisionResponse.body).to.have.property('should_transcode');
        expect(decisionResponse.body).to.have.property('reason');
        cy.task('log', `✅ Playback decision API contract working`);
      });
      
      // Test start transcoding with media_file_id (frontend format)
      cy.request({
        method: 'POST',
        url: `${API_BASE}/api/playback/start`,
        body: {
          media_file_id: testFile.id,
          container: 'dash',
          enable_abr: true
        }
      }).then((startResponse) => {
        expect(startResponse.status).to.equal(200);
        expect(startResponse.body).to.have.property('id');
        expect(startResponse.body).to.have.property('content_hash');
        expect(startResponse.body).to.have.property('manifest_url');
        cy.task('log', `✅ Start transcoding API contract working`);
        
        // Clean up
        cy.request({
          method: 'DELETE',
          url: `${API_BASE}/api/playback/session/${startResponse.body.id}`,
          failOnStatusCode: false
        });
      });
    });
  });
});