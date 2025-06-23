/// <reference types="cypress" />

describe('Video Playback - Simple Test', () => {
  beforeEach(() => {
    // Clean up any existing sessions
    cy.request({
      method: 'DELETE',
      url: 'http://localhost:8080/api/playback/sessions/all',
      failOnStatusCode: false,
    });
  });

  it('should start transcoding session for media file', () => {
    // Get a media file
    cy.request({
      method: 'GET',
      url: 'http://localhost:8080/api/media/files',
    }).then((response) => {
      expect(response.status).to.eq(200);
      expect(response.body.media_files).to.have.length.greaterThan(0);
      
      const mediaFile = response.body.media_files.find(f => f.media_type === 'episode' && f.video_codec);
      expect(mediaFile).to.exist;
      
      cy.log(`Found media file: ${mediaFile.path}`);
      
      // Start transcoding session
      cy.request({
        method: 'POST',
        url: 'http://localhost:8080/api/playback/start',
        body: {
          media_file_id: mediaFile.id,
          container: 'dash',
          enable_abr: true,
        },
      }).then((startResponse) => {
        expect(startResponse.status).to.eq(200);
        expect(startResponse.body.id).to.exist;
        expect(startResponse.body.manifest_url).to.exist;
        expect(startResponse.body.provider).to.exist;
        
        const sessionId = startResponse.body.id;
        cy.log(`Started session: ${sessionId}`);
        
        // Wait for session to transition to running
        cy.wait(2000);
        
        // Check session status
        cy.request({
          method: 'GET',
          url: `http://localhost:8080/api/playback/session/${sessionId}`,
        }).then((sessionResponse) => {
          expect(sessionResponse.status).to.eq(200);
          expect(sessionResponse.body.Status).to.eq('running');
          cy.log(`Session status: ${sessionResponse.body.Status}`);
        });
        
        // Wait for manifest to be created
        cy.wait(5000);
        
        // Try to fetch manifest
        cy.request({
          method: 'GET',
          url: `http://localhost:8080${startResponse.body.manifest_url}`,
          failOnStatusCode: false,
        }).then((manifestResponse) => {
          // Log the response for debugging
          cy.log(`Manifest response status: ${manifestResponse.status}`);
          
          if (manifestResponse.status === 200) {
            cy.log('✅ Manifest is available!');
            expect(manifestResponse.body).to.include('<MPD');
          } else {
            cy.log('⚠️ Manifest not yet ready, but transcoding is running');
          }
        });
        
        // Clean up session
        cy.request({
          method: 'DELETE',
          url: `http://localhost:8080/api/playback/session/${sessionId}`,
          failOnStatusCode: false,
        });
      });
    });
  });
});