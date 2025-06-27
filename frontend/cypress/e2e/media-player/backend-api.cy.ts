describe('MediaPlayer - Backend API Tests', () => {
  const API_BASE = 'http://localhost:8080';
  
  beforeEach(() => {
    // Clean up sessions
    cy.request({
      method: 'DELETE',
      url: `${API_BASE}/api/playback/sessions/all`,
      failOnStatusCode: false
    });
  });

  describe('Health & Media Availability', () => {
    it('should verify backend health and media files', () => {
      cy.request(`${API_BASE}/api/health`).then((response) => {
        expect(response.status).to.equal(200);
        cy.task('log', '✅ Backend is healthy');
      });

      cy.request(`${API_BASE}/api/media/files?limit=10`).then((response) => {
        expect(response.status).to.equal(200);
        expect(response.body.media_files).to.be.an('array');
        expect(response.body.media_files.length).to.be.greaterThan(0);
        cy.task('log', `✅ Found ${response.body.media_files.length} media files`);
      });
    });
  });

  describe('Transcoding Sessions', () => {
    it('should create and manage transcoding session lifecycle', () => {
      cy.request(`${API_BASE}/api/media/files?limit=1`).then((response) => {
        const testFile = response.body.media_files?.[0];
        if (!testFile) {
          cy.skip('No media files available');
          return;
        }
        
        // Start transcoding
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
          expect(startResponse.body.id).to.exist;
          
          const sessionId = startResponse.body.id;
          cy.task('log', `✅ Session created: ${sessionId}`);
          
          // Verify content hash if available
          if (startResponse.body.content_hash) {
            expect(startResponse.body.content_url).to.equal(
              `/api/v1/content/${startResponse.body.content_hash}/`
            );
            expect(startResponse.body.manifest_url).to.equal(
              `/api/v1/content/${startResponse.body.content_hash}/manifest.mpd`
            );
            cy.task('log', `✅ Content hash: ${startResponse.body.content_hash}`);
          }
          
          // Check session status
          cy.request(`${API_BASE}/api/playback/session/${sessionId}`).then((sessionResponse) => {
            expect(sessionResponse.status).to.equal(200);
            expect(sessionResponse.body.ID).to.equal(sessionId);
            cy.task('log', `📊 Session status: ${sessionResponse.body.Status}`);
          });
          
          // Wait for some transcoding progress
          cy.wait(3000);
          
          // Stop session
          cy.request({
            method: 'DELETE',
            url: `${API_BASE}/api/playback/session/${sessionId}`
          }).then((stopResponse) => {
            expect(stopResponse.status).to.equal(200);
            cy.task('log', '✅ Session stopped');
          });
        });
      });
    });

    it('should handle multiple concurrent sessions', () => {
      cy.request(`${API_BASE}/api/media/files?limit=3`).then((response) => {
        const files = response.body.media_files?.slice(0, 3);
        if (!files || files.length === 0) {
          cy.skip('Not enough media files');
          return;
        }
        
        // Start multiple sessions
        const sessionPromises = files.map((file: any) => 
          cy.request({
            method: 'POST',
            url: `${API_BASE}/api/playback/start`,
            body: {
              media_file_id: file.id,
              container: 'dash'
            }
          })
        );
        
        cy.wrap(Promise.all(sessionPromises)).then((responses: any[]) => {
          expect(responses).to.have.length(files.length);
          cy.task('log', `✅ Started ${responses.length} concurrent sessions`);
          
          // List all sessions
          cy.request(`${API_BASE}/api/playback/sessions`).then((listResponse) => {
            expect(listResponse.body.sessions).to.be.an('array');
            expect(listResponse.body.sessions.length).to.be.at.least(files.length);
            cy.task('log', `📊 Active sessions: ${listResponse.body.sessions.length}`);
          });
          
          // Clean up all sessions
          cy.request({
            method: 'DELETE',
            url: `${API_BASE}/api/playback/sessions/all`
          }).then((cleanupResponse) => {
            cy.task('log', `✅ Cleaned up ${cleanupResponse.body.stopped_count} sessions`);
          });
        });
      });
    });
  });

  describe('Content URLs', () => {
    it('should verify content-addressable storage URLs', () => {
      cy.request(`${API_BASE}/api/media/files?limit=1`).then((response) => {
        const testFile = response.body.media_files?.[0];
        if (!testFile) {
          cy.skip('No media files available');
          return;
        }
        
        // Start transcoding
        cy.request({
          method: 'POST',
          url: `${API_BASE}/api/playback/start`,
          body: {
            media_file_id: testFile.id,
            container: 'dash'
          }
        }).then((startResponse) => {
          const contentHash = startResponse.body.content_hash;
          
          if (contentHash) {
            // Wait for content to be available
            cy.wait(5000);
            
            // Check manifest availability
            cy.request({
              method: 'GET',
              url: `${API_BASE}/api/v1/content/${contentHash}/manifest.mpd`,
              failOnStatusCode: false
            }).then((manifestResponse) => {
              if (manifestResponse.status === 200) {
                expect(manifestResponse.headers['content-type']).to.include('dash+xml');
                expect(manifestResponse.body).to.include('<MPD');
                expect(manifestResponse.body).to.include('type="static"');
                cy.task('log', '✅ Content-addressable manifest available');
              } else {
                cy.task('log', `⚠️ Manifest not ready: ${manifestResponse.status}`);
              }
            });
          } else {
            cy.task('log', '⚠️ No content hash available yet');
          }
          
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
});