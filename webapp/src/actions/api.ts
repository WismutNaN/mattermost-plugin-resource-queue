const PLUGIN_ID = 'com.scientia.resource-queue';

function apiUrl(path: string): string {
    return `/plugins/${PLUGIN_ID}/api/v1${path}`;
}

function getCsrfToken(): string {
    const match = document.cookie.match(/MMCSRF=([^\s;]+)/);
    return match ? match[1] : '';
}

async function doFetch(url: string, options: RequestInit = {}): Promise<any> {
    const csrfToken = getCsrfToken();
    const resp = await fetch(url, {
        credentials: 'same-origin',
        ...options,
        headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest',
            ...(csrfToken ? {'X-CSRF-Token': csrfToken} : {}),
            ...((options as any).headers || {}),
        },
    });
    if (!resp.ok) {
        const body = await resp.json().catch(() => ({error: resp.statusText}));
        throw new Error(body.error || resp.statusText);
    }
    return resp.json();
}

export async function getAllStatus() {
    return doFetch(apiUrl('/status'));
}

export async function getResourceStatus(id: string) {
    return doFetch(apiUrl(`/status/${id}`));
}

export async function getResources() {
    return doFetch(apiUrl('/resources'));
}

export async function createResource(data: any) {
    return doFetch(apiUrl('/resources'), {method: 'POST', body: JSON.stringify(data)});
}

export async function updateResource(id: string, data: any) {
    return doFetch(apiUrl(`/resources/${id}`), {method: 'PUT', body: JSON.stringify(data)});
}

export async function deleteResource(id: string) {
    return doFetch(apiUrl(`/resources/${id}`), {method: 'DELETE'});
}

export async function bookResource(id: string, minutes: number, purpose: string = '') {
    return doFetch(apiUrl(`/resources/${id}/book`), {
        method: 'POST',
        body: JSON.stringify({minutes, purpose}),
    });
}

export async function releaseResource(id: string) {
    return doFetch(apiUrl(`/resources/${id}/release`), {method: 'POST'});
}

export async function extendResource(id: string, minutes: number) {
    return doFetch(apiUrl(`/resources/${id}/extend`), {
        method: 'POST',
        body: JSON.stringify({minutes}),
    });
}

export async function joinQueue(id: string, minutes: number, purpose: string = '') {
    return doFetch(apiUrl(`/resources/${id}/queue`), {
        method: 'POST',
        body: JSON.stringify({minutes, purpose}),
    });
}

export async function leaveQueue(id: string) {
    return doFetch(apiUrl(`/resources/${id}/queue`), {method: 'DELETE'});
}

export async function subscribeResource(id: string) {
    return doFetch(apiUrl(`/resources/${id}/subscribe`), {method: 'POST'});
}

export async function unsubscribeResource(id: string) {
    return doFetch(apiUrl(`/resources/${id}/unsubscribe`), {method: 'POST'});
}

export async function getHistory(id: string) {
    return doFetch(apiUrl(`/resources/${id}/history`));
}

export async function getPresets() {
    return doFetch(apiUrl('/presets'));
}
