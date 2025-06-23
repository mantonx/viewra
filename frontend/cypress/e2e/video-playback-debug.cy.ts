/// <reference types="cypress" />

describe('Video Playback Debug', () => {
  it('should debug episode player loading', () => {
    // Get an episode
    cy.request({
      method: 'GET',
      url: 'http://localhost:8080/api/media/files?limit=100',
    }).then((response) => {
      const episode = response.body.media_files.find(f => 
        f.media_type === 'episode' && f.video_codec && f.media_id
      );
      
      cy.log(`Episode media_id: ${episode.media_id}`);
      cy.log(`Episode file id: ${episode.id}`);
      
      // First check if we can get the file by media_id (which is what the episode player uses)
      cy.request({
        method: 'GET',
        url: `http://localhost:8080/api/media/files/${episode.media_id}`,
        failOnStatusCode: false,
      }).then((fileResponse) => {
        cy.log(`File by media_id response: ${fileResponse.status}`);
        
        if (fileResponse.status === 404) {
          cy.log('❌ Cannot find file by media_id, will need to search');
          
          // Check if search endpoint works
          cy.request({
            method: 'GET',
            url: 'http://localhost:8080/api/media/',
            qs: { limit: 50000 },
          }).then((searchResponse) => {
            cy.log(`Search response status: ${searchResponse.status}`);
            cy.log(`Total media files: ${searchResponse.body.media?.length || 0}`);
            
            const found = searchResponse.body.media?.find(
              (file: any) => file.media_id === episode.media_id && file.media_type === 'episode'
            );
            
            if (found) {
              cy.log(`✅ Found episode in search: ${found.id}`);
            } else {
              cy.log('❌ Episode not found in search');
            }
          });
        }
      });
      
      // Check metadata endpoint
      cy.request({
        method: 'GET',
        url: `http://localhost:8080/api/media/files/${episode.id}/metadata`,
        failOnStatusCode: false,
      }).then((metadataResponse) => {
        cy.log(`Metadata response: ${metadataResponse.status}`);
        if (metadataResponse.status === 200) {
          cy.log('Metadata:', JSON.stringify(metadataResponse.body, null, 2));
        }
      });
      
      // Now visit the page and check network activity
      cy.visit(`/player/episode/${episode.media_id}`, {
        onBeforeLoad(win) {
          // Stub console methods to capture logs
          cy.stub(win.console, 'log').as('consoleLog');
          cy.stub(win.console, 'error').as('consoleError');
        }
      });
      
      // Wait a bit for network activity
      cy.wait(2000);
      
      // Check console logs
      cy.get('@consoleLog').then((stub) => {
        const calls = (stub as any).getCalls();
        calls.forEach((call: any) => {
          cy.log(`Console log: ${call.args.join(' ')}`);
        });
      });
      
      cy.get('@consoleError').then((stub) => {
        const calls = (stub as any).getCalls();
        calls.forEach((call: any) => {
          cy.log(`Console error: ${call.args.join(' ')}`);
        });
      });
    });
  });
});