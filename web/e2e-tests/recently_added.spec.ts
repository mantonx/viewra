const { test, expect } = require('@playwright/test');

test.describe('Recently Added', () => {
  test.beforeEach(async ({ page }) => {
    // Login
    await page.goto('http://localhost:5173/login');
    await page.fill('input[name="username"]', 'dev');
    await page.fill('input[name="password"]', 'dev');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/');
  });

  test('recently added section shows correct order', async ({ page }) => {
    // Go to home page
    await page.goto('http://localhost:5173/');
    
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Take a screenshot for debugging
    await page.screenshot({ path: '/home/fictional/Projects/viewra/web/e2e-tests/recently_added.png', fullPage: true });
    
    // Print the page content to see what's there
    const content = await page.content();
    console.log('Page has Recently Added:', content.includes('Recently Added'));
    console.log('Page has Springsteen:', content.includes('Springsteen'));
    
    // Find the recently added row
    const homeContent = await page.textContent('body');
    console.log('First 3000 chars of body:', homeContent?.slice(0, 3000));
  });
});
