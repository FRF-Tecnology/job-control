import type { Page } from '@playwright/test';

// A preferência explícita do app prevalece sobre a aparência do sistema.
export async function temaDeTeste(page: Page, tema: 'light' | 'dark') {
  await page.emulateMedia({ colorScheme: tema });
  await page.addInitScript(t => localStorage.setItem('tema', t), tema === 'light' ? 'claro' : 'escuro');
}
