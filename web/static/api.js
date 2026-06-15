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
            throw new Error('Сессия истекла. Пожалуйста, авторизуйтесь заново.');
        }

        // Пробуем получить JSON ответ
        let data;
        const contentType = response.headers.get('content-type');
        if (contentType && contentType.includes('application/json')) {
            data = await response.json();
        } else {
            // Если ответ не JSON, читаем как текст
            const text = await response.text();
            throw new Error(`Ошибка сервера: ${response.status}`);
        }

        if (!response.ok) {
            // Извлекаем сообщение об ошибке из ответа сервера
            const errorMessage = data.error || data.message || 'Произошла ошибка';
            throw new Error(errorMessage);
        }

        return data;
    }
};

export default api;