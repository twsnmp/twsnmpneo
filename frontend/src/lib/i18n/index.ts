import { init, register, getLocaleFromNavigator, locale, addMessages } from 'svelte-i18n';
import ja from '../../locales/ja.json';
import en from '../../locales/en.json';

// Synchronously register base translations to prevent flash of untranslated content (FOUC)
addMessages('ja', ja);
addMessages('en', en);

export type SupportedLocale = 'ja' | 'en';

const STORAGE_KEY = 'twsnmp_locale';

export function getSavedLocale(): SupportedLocale {
  if (typeof window === 'undefined') return 'ja';
  const saved = localStorage.getItem(STORAGE_KEY) as SupportedLocale | null;
  if (saved === 'ja' || saved === 'en') {
    return saved;
  }
  const navLang = getLocaleFromNavigator();
  if (navLang && navLang.startsWith('en')) {
    return 'en';
  }
  return 'ja';
}

export function switchLocale(newLocale: SupportedLocale) {
  locale.set(newLocale);
  if (typeof window !== 'undefined') {
    localStorage.setItem(STORAGE_KEY, newLocale);
    document.documentElement.lang = newLocale;
  }
}

export function setupI18n() {
  const initial = getSavedLocale();
  if (typeof window !== 'undefined') {
    document.documentElement.lang = initial;
  }
  init({
    fallbackLocale: 'ja',
    initialLocale: initial,
  });
}

// Auto-initialize
setupI18n();

export { locale };
