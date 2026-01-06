// Shared API helper
export const api = {
    async request(url, options = {}) {
        const response = await fetch(url, {
            ...options,
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            }
        });

        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            if (response.status === 401) throw new Error('Unauthorized: Please check your API token.');
            if (response.status === 403) throw new Error('Forbidden: You do not have permission to perform this action.');
            if (response.status === 409) throw new Error('Conflict: An item with this name already exists.');
            if (response.status === 400) throw new Error(error.error || 'Invalid data provided. Please check your inputs.');

            throw new Error(error.error || `Request failed with status ${response.status}`);
        }

        // For 204 No Content
        if (response.status === 204) return null;

        return await response.json();
    },
    get(url) { return this.request(url); },
    post(url, data) { return this.request(url, { method: 'POST', body: JSON.stringify(data) }); },
    put(url, data) { return this.request(url, { method: 'PUT', body: JSON.stringify(data) }); },
    delete(url) { return this.request(url, { method: 'DELETE' }); }
};