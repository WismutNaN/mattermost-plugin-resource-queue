import React from 'react';
import RHSView from './components/RHSView';

const PLUGIN_ID = 'com.scientia.resource-queue';

const PluginIcon = () => (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
        stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"
        style={{width: '18px', height: '18px'}}>
        <rect x="2" y="3" width="16" height="11" rx="1.5"/>
        <line x1="6" y1="17" x2="14" y2="17"/>
        <line x1="10" y1="14" x2="10" y2="17"/>
        <circle cx="8" cy="7.5" r="1" fill="currentColor" stroke="none"/>
        <circle cx="12" cy="7.5" r="1" fill="currentColor" stroke="none"/>
        <circle cx="19.5" cy="8" r="4"/>
        <line x1="19.5" y1="6" x2="19.5" y2="8"/>
        <line x1="19.5" y1="8" x2="21" y2="9.2"/>
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
