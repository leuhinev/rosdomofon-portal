import { getToken, logout } from './auth.js';

const api = {
    async request(endpoint, options = {}) {
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };
        const token = getToken();
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const response = await fetch(endpoint, { ...options, headers });

        if (response.status === 401) {
            logout();
            throw new Error('unauthorized');
        }
        const data = await response.json();
        if (!response.ok) {
            throw new Error(data.error || 'request failed');
        }
        return data;
    }
};

export default api;