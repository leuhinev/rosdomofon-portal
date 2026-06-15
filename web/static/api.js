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

        // Обработка 401 - не авторизован
        if (response.status === 401) {
            logout();
            throw new Error('Сессия истекла. Пожалуйста, авторизуйтесь заново.');
        }

        // Парсим JSON ответ
        let data;
        try {
            data = await response.json();
        } catch (e) {
            // Если ответ не JSON (например, HTML ошибка)
            throw new Error(`Ошибка сервера: ${response.status} ${response.statusText}`);
        }

        // Если статус не 2xx - ошибка
        if (!response.ok) {
            const errorMessage = data.error || data.message || 'Произошла ошибка';
            throw new Error(errorMessage);
        }

        // Успешный ответ
        return data;
    }
};

export default api;