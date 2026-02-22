import React from 'react';
import RHSView from './components/RHSView';

const PLUGIN_ID = 'com.scientia.resource-queue';

const PluginIcon = () => (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
        stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"
        style={{width: '18px', height: '18px'}}>
        {/* Монитор */}
        <rect x="3" y="4" width="18" height="12" rx="2" />
        <line x1="12" y1="16" x2="12" y2="20" />
        <line x1="8" y1="20" x2="16" y2="20" />
        
        {/* Список очереди внутри */}
        <circle cx="7" cy="9" r="1" fill="currentColor" stroke="none" />
        <line x1="10" y1="9" x2="17" y2="9" />
        
        <circle cx="7" cy="13" r="1" fill="currentColor" stroke="none" />
        <line x1="10" y1="13" x2="17" y2="13" />
    </svg>
);

class PluginClass {
    rhsRegistration: any = null;

    initialize(registry: any, store: any) {
        const {toggleRHSPlugin} = registry.registerRightHandSidebarComponent(
            (props: any) => {
                const state = store.getState();
                const theme = getTheme(state);
                return React.createElement(RHSView, {...props, theme});
            },
            'Resource Queue'
        );

        this.rhsRegistration = toggleRHSPlugin;

        registry.registerChannelHeaderButtonAction(
            React.createElement(PluginIcon),
            () => store.dispatch(toggleRHSPlugin),
            null,
            'Resource Queue'
        );
    }

    uninitialize() {
        this.rhsRegistration = null;
    }
}

function getTheme(state: any): any {
    try {
        const prefs = state?.entities?.preferences?.myPreferences;
        if (!prefs) return {};
        const themeKey = Object.keys(prefs).find((k: string) => k.includes('theme'));
        if (themeKey && prefs[themeKey]?.value) {
            return JSON.parse(prefs[themeKey].value);
        }
    } catch {}
    return {};
}

(window as any).registerPlugin(PLUGIN_ID, new PluginClass());
