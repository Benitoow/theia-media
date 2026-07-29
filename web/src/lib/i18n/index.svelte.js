import { browser } from '$app/environment';
import { catalogs, defaultLocale } from './locales/index.js';

const storageKey = 'theia.locale';

// Object.hasOwn is newer than several television browser engines Theia still
// needs to tolerate. Keep locale validation boring and compatible.
const hasLocale = (code) => Object.prototype.hasOwnProperty.call(catalogs, code);

const available = Object.freeze(
	Object.values(catalogs).map(({ metadata }) =>
		Object.freeze({
			code: metadata.code,
			label: metadata.label,
			lang: metadata.htmlLang
		})
	)
);

class I18n {
	current = $state(defaultLocale);
	ready = $state(false);

	get locale() {
		return this.current;
	}

	get localeTag() {
		return catalogs[this.current].metadata.localeTag;
	}

	get htmlLang() {
		return catalogs[this.current].metadata.htmlLang;
	}

	get t() {
		return catalogs[this.current].strings;
	}

	get available() {
		return available;
	}

	bootstrap() {
		if (this.ready) {
			this.syncDocumentLanguage();
			return this.current;
		}

		if (browser) {
			try {
				const stored = localStorage.getItem(storageKey);
				if (stored && hasLocale(stored)) this.current = stored;
			} catch {
				// Some privacy modes deny local storage. French remains the
				// default and language switching still works for this page.
			}
		}

		this.syncDocumentLanguage();
		this.ready = true;
		return this.current;
	}

	setLocale(code) {
		if (!hasLocale(code)) return false;

		this.current = code;
		this.syncDocumentLanguage();

		if (browser) {
			try {
				localStorage.setItem(storageKey, code);
			} catch {
				// The reactive session still changes even if persistence is
				// blocked by the browser.
			}
		}

		return true;
	}

	formatUptime(seconds) {
		return catalogs[this.current].formatUptime(seconds);
	}

	formatDecimal(value) {
		return catalogs[this.current].formatDecimal(value);
	}

	formatSize(bytes) {
		return catalogs[this.current].formatSize(bytes);
	}

	formatRuntime(minutes) {
		return catalogs[this.current].formatRuntime(minutes);
	}

	syncDocumentLanguage() {
		if (!browser) return;
		document.documentElement.lang = this.htmlLang;
	}
}

export const i18n = new I18n();

// Existing components can migrate from the old static module without
// changing every `t.foo` access at once. Each property read resolves against
// the active catalogue and therefore participates in Svelte reactivity.
export const strings = new Proxy(
	{},
	{
		get(_target, property) {
			return i18n.t[property];
		},
		has(_target, property) {
			return property in i18n.t;
		},
		ownKeys() {
			return Reflect.ownKeys(i18n.t);
		},
		getOwnPropertyDescriptor(_target, property) {
			if (!(property in i18n.t)) return undefined;
			return {
				configurable: true,
				enumerable: true,
				value: i18n.t[property]
			};
		}
	}
);

export function bootstrapLocale() {
	return i18n.bootstrap();
}

export function setLocale(code) {
	return i18n.setLocale(code);
}

export function formatUptime(seconds) {
	return i18n.formatUptime(seconds);
}

export function formatDecimal(value) {
	return i18n.formatDecimal(value);
}

export function formatSize(bytes) {
	return i18n.formatSize(bytes);
}

export function formatRuntime(minutes) {
	return i18n.formatRuntime(minutes);
}

export { available as availableLocales };

export default i18n;
