// Custom commands for video player testing

declare namespace Cypress {
  interface Chainable {
    /**
     * Custom command to start video playback with mocked backend
     */
    startVideoPlayback(videoId: string, options?: {
      shouldSucceed?: boolean;
      sessionId?: string;
      container?: string;
    }): Chainable<void>;

    /**
     * Custom command to wait for video player to be ready
     */
    waitForPlayerReady(): Chainable<void>;

    /**
     * Custom command to check for FFmpeg processes (debugging)
     */
    checkFFmpegProcesses(): Chainable<void>;

    /**
     * Custom command to cleanup all sessions
     */
    cleanupAllSessions(): Chainable<void>;
  }
}

Cypress.Commands.add('startVideoPlayback', (videoId: string, options = {}) => {
  const {
    shouldSucceed = true,
    sessionId = `test-session-${Date.now()}`,
    container = 'dash'
  } = options;

  if (shouldSucceed) {
    // Mock successful transcoding
    cy.intercept('POST', '/api/playback/start', {
      statusCode: 200,
      body: {
        id: sessionId,
        status: 'queued',
        manifest_url: `/api/playback/stream/${sessionId}/manifest.mpd`,
        provider: 'ffmpeg_software'
      }
    }).as('startTranscode');

    // Mock manifest
    cy.intercept('GET', `/api/playback/stream/${sessionId}/manifest.mpd`, {
      fixture: 'manifest.mpd'
    }).as('getManifest');

    // Mock session info
    cy.intercept('GET', `/api/playback/sessions/${sessionId}`, {
      statusCode: 200,
      body: {
        id: sessionId,
        status: 'running',
        provider: 'ffmpeg_software'
      }
    }).as('getSession');
  } else {
    // Mock failed transcoding
    cy.intercept('POST', '/api/playback/start', {
      statusCode: 500,
      body: { error: 'no transcoding providers available' }
    }).as('startTranscodeError');
  }

  // Visit the video page
  cy.visit(`/video/${videoId}`);
  
  // Click play
  cy.get('[data-testid="play-button"]').click();

  if (shouldSucceed) {
    cy.wait('@startTranscode');
    cy.wait('@getManifest');
  } else {
    cy.wait('@startTranscodeError');
  }
});

Cypress.Commands.add('waitForPlayerReady', () => {
  // Wait for media player to be visible
  cy.get('[data-testid="media-player"]', { timeout: 10000 }).should('be.visible');
  
  // Wait for video element
  cy.get('video').should('be.visible');
  
  // Ensure not in loading state
  cy.get('[data-testid="loading-indicator"]').should('not.exist');
});

Cypress.Commands.add('checkFFmpegProcesses', () => {
  // Make API call to check for running processes
  cy.request({
    method: 'GET',
    url: `${Cypress.env('apiUrl')}/api/playback/sessions`,
    failOnStatusCode: false
  }).then((response) => {
    if (response.status === 200 && response.body.sessions) {
      const activeSessions = response.body.sessions.filter(
        (s: any) => s.status === 'running' || s.status === 'queued'
      );
      
      // Log active sessions for debugging
      cy.task('log', `Active sessions: ${activeSessions.length}`);
      activeSessions.forEach((session: any) => {
        cy.task('log', `Session ${session.id}: ${session.status} (${session.provider})`);
      });
    }
  });
});

Cypress.Commands.add('cleanupAllSessions', () => {
  // Call the cleanup endpoint
  cy.request({
    method: 'DELETE',
    url: `${Cypress.env('apiUrl')}/api/playback/sessions/all`,
    failOnStatusCode: false
  }).then((response) => {
    cy.task('log', `Cleanup response: ${JSON.stringify(response.body)}`);
  });
});

// Additional custom commands for specific debugging

// Command to simulate the exact error scenario
Cypress.Commands.add('simulatePlayerError', (errorCode: number = 4000) => {
  cy.window().then((win) => {
    // Simulate Shaka Player error
    const error = {
      severity: 2,
      category: 4,
      code: errorCode,
      data: ['DASH_MANIFEST_UNKNOWN', 'Unknown DASH manifest type'],
      handled: false
    };
    
    // Dispatch error event
    const errorEvent = new CustomEvent('shaka-error', { detail: error });
    win.dispatchEvent(errorEvent);
  });
});

// Command to check video element state
Cypress.Commands.add('checkVideoState', () => {
  cy.get('video').then(($video) => {
    const video = $video[0] as HTMLVideoElement;
    
    cy.task('log', `Video state: readyState=${video.readyState}, networkState=${video.networkState}`);
    cy.task('log', `Video src: ${video.src || 'none'}`);
    cy.task('log', `Video currentTime: ${video.currentTime}`);
    cy.task('log', `Video duration: ${video.duration}`);
    cy.task('log', `Video paused: ${video.paused}`);
    cy.task('log', `Video ended: ${video.ended}`);
    cy.task('log', `Video error: ${video.error?.message || 'none'}`);
  });
});