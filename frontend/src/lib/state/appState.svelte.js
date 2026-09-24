import { DerivedState } from './derivedState.svelte.js';
import { attachActionsPart1 } from './actionsPart1.js';
import { attachActionsPart2 } from './actionsPart2.js';
import { attachActionsPart3 } from './actionsPart3.js';
import { attachActionsPart4 } from './actionsPart4.js';
import { attachActionsPart5 } from './actionsPart5.js';
import { attachActionsPart6 } from './actionsPart6.js';
import { attachActionsPart7 } from './actionsPart7.js';
import { attachActionsPart8 } from './actionsPart8.js';

export const app = new DerivedState();
attachActionsPart1(app);
attachActionsPart2(app);
attachActionsPart3(app);
attachActionsPart4(app);
attachActionsPart5(app);
attachActionsPart6(app);
attachActionsPart7(app);
attachActionsPart8(app);

if (typeof window !== 'undefined') {
  window.app = app;
}

app.initOnMount = () => {
    // Theme setup
    const savedTheme = localStorage.getItem('gdrive_theme') || 'dark';
    app.currentTheme = savedTheme;
    document.documentElement.setAttribute('data-theme', savedTheme);

    // Column widths setup
    try {
      const savedWidths = localStorage.getItem('gdrive_col_widths');
      if (savedWidths) {
        Object.assign(app.colWidths, JSON.parse(savedWidths));
        if (!app.colWidths.prog || app.colWidths.prog < 150) {
          app.colWidths.prog = 160;
        }
      }
    } catch (e) {}

    app.checkAuthStatus();

    const handleOAuthMessage = (event) => {
      if (event.data && event.data.type === 'gdrive-oauth-success') {
        app.fetchOAuthStatus();
        app.oauthMessage = `Google Account ${event.data.email || ''} connected successfully!`;
        setTimeout(() => { app.oauthMessage = ''; }, 5000);
      }
    };
    window.addEventListener('message', handleOAuthMessage);

    window.addEventListener('keydown', app.handleKeyDown);
    window.addEventListener('click', app.closeContextMenu);
    window.addEventListener('click', app.closeMenus);
};

app.cleanupOnDestroy = () => {
    if (app.eventSource) app.eventSource.close();
    if (app.pollInterval) clearInterval(app.pollInterval);
    window.removeEventListener('keydown', app.handleKeyDown);
    window.removeEventListener('click', app.closeContextMenu);
    window.removeEventListener('click', app.closeMenus);
};
