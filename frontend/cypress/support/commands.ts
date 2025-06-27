// Custom commands for MediaPlayer testing

declare namespace Cypress {
  interface Chainable {
    /**
     * Clean up all transcoding sessions
     */
    cleanupSessions(): Chainable<void>;

    /**
     * Check video element state for debugging
     */
    checkVideoState(): Chainable<void>;

    /**
     * Wait for media player to be ready
     */
    waitForPlayerReady(): Chainable<void>;

    /**
     * Mock successful playback start
     */
    mockPlaybackStart(options?: {
      sessionId?: string;
      contentHash?: string;
      container?: string;
    }): void;

    /**
     * Check active transcoding sessions
     */
    checkActiveSessions(): Chainable<void>;
  }
}

Cypress.Commands.add('cleanupSessions', () => {
  cy.request({
    method: 'DELETE',
    url: 'http://localhost:8080/api/playback/sessions/all',
    failOnStatusCode: false
  }).then((response) => {
    if (response.body?.stopped_count) {
      cy.task('log', `Cleaned up ${response.body.stopped_count} sessions`);
    }
  });
});

Cypress.Commands.add('checkVideoState', () => {
  cy.get('video').then(($video) => {
    const video = $video[0] as HTMLVideoElement;
    
    const state = {
      readyState: video.readyState,
      networkState: video.networkState,
      src: video.src || 'none',
      currentTime: video.currentTime,
      duration: video.duration,
      paused: video.paused,
      ended: video.ended,
      error: video.error?.message || 'none'
    };
    
    cy.task('log', `Video state: ${JSON.stringify(state, null, 2)}`);
  });
});

Cypress.Commands.add('waitForPlayerReady', () => {
  // Wait for media player container
  cy.get('[data-testid="media-player"]', { timeout: 15000 }).should('be.visible');
  
  // Ensure not in loading state
  cy.get('[data-testid="loading-indicator"]', { timeout: 5000 }).should('not.exist');
});

Cypress.Commands.add('mockPlaybackStart', (options = {}) => {
  const {
    sessionId = `test-session-${Date.now()}`,
    contentHash = `test-hash-${Date.now()}`,
    container = 'dash'
  } = options;

  // Mock successful transcoding start with content-addressable storage
  cy.intercept('POST', '**/api/playback/start', {
    statusCode: 200,
    body: {
      id: sessionId,
      status: 'queued',
      provider: 'ffmpeg-pipeline',
      content_hash: contentHash,
      content_url: `/api/v1/content/${contentHash}/`,
      manifest_url: `/api/v1/content/${contentHash}/manifest.${container === 'hls' ? 'm3u8' : 'mpd'}`
    }
  }).as('startPlayback');

  // Mock content manifest
  const manifestUrl = container === 'hls' 
    ? `**/api/v1/content/${contentHash}/playlist.m3u8`
    : `**/api/v1/content/${contentHash}/manifest.mpd`;
  
  cy.intercept('GET', manifestUrl, {
    statusCode: 200,
    headers: { 
      'Content-Type': container === 'hls' 
        ? 'application/vnd.apple.mpegurl' 
        : 'application/dash+xml' 
    },
    fixture: container === 'hls' ? 'playlist.m3u8' : 'manifest.mpd'
  }).as('getManifest');

  // Mock session status
  cy.intercept('GET', `**/api/playback/session/${sessionId}`, {
    statusCode: 200,
    body: {
      ID: sessionId,
      Status: 'running',
      Provider: 'ffmpeg-pipeline',
      ContentHash: contentHash
    }
  }).as('getSession');
});

Cypress.Commands.add('checkActiveSessions', () => {
  cy.request({
    method: 'GET',
    url: 'http://localhost:8080/api/playback/sessions',
    failOnStatusCode: false
  }).then((response) => {
    if (response.status === 200 && response.body.sessions) {
      const activeSessions = response.body.sessions.filter(
        (s: any) => s.Status === 'running' || s.Status === 'queued'
      );
      
      cy.task('log', `Active sessions: ${activeSessions.length}`);
      activeSessions.forEach((session: any) => {
        cy.task('log', `  - ${session.ID}: ${session.Status} (${session.Provider})`);
      });
    }
  });
});