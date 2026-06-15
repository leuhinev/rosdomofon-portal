import api from './api.js';
import { showMessage } from './auth.js';
import { showConfirm, closeModal } from './modal.js';

let flats = [];
let cachedKeys = []; // Кешируем ключи

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function maskKey(key) {
    if (!key) return '';
    if (key.length <= 8) return '****' + key.slice(-4);
    return key.slice(0, 6) + '...' + key.slice(-4);
}

function setFlats(flatsData) {
    flats = flatsData;
}

async function loadKeys() {
    try {
        const keys = await api.request('/api/keys');
        cachedKeys = keys; // Сохраняем в кеш
        const container = document.getElementById('keys-list');
        if (keys.length === 0) {
            container.innerHTML = '<p class="empty-message">🔐 Нет добавленных ключей</p>';
            return;
        }
        container.innerHTML = keys.map(key => `
            <div class="key-item" data-id="${key.ID}">
                <div class="key-header">
                    <span class="key-data">🔑 ${maskKey(key.KeyData)}</span>
                    <div class="actions">
                        <button class="edit-btn" onclick="window.editKey(${key.ID})">✏️ Редактировать</button>
                        <button class="delete-btn" onclick="window.confirmDeleteKey(${key.ID})">🗑️ Удалить</button>
                    </div>
                </div>
                ${key.Comment ? `<div class="comment">💬 ${escapeHtml(key.Comment)}</div>` : ''}
                <div class="address">📍 ${flats.find(f => f.flat_id === key.FlatID)?.address || 'Адрес не найден'}</div>
            </div>
        `).join('');
    } catch (err) {
        console.error('Error loading keys:', err);
        document.getElementById('keys-list').innerHTML = '<p class="error-message">❌ Ошибка загрузки ключей</p>';
    }
}

// Функция editKey - использует кешированные данные
async function editKey(id) {
    const key = cachedKeys.find(k => k.ID === id);
    if (!key) {
        console.warn('Cache miss, loading keys again');
        await loadKeys();
        const freshKey = cachedKeys.find(k => k.ID === id);
        if (!freshKey) {
            showMessage('Ключ не найден');
            return;
        }
        showEditKeyModal(freshKey);
        return;
    }
    showEditKeyModal(key);
}

function showEditKeyModal(key) {
    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
        <h3>✏️ Редактировать ключ</h3>
        <form id="edit-key-form">
            <div class="form-group">
                <label for="key_data">Ключ (HEX):</label>
                <input type="text" id="key_data" value="${key.KeyData}" required>
            </div>
            <div class="form-group">
                <label for="comment">Комментарий:</label>
                <textarea id="comment" rows="2">${escapeHtml(key.Comment || '')}</textarea>
            </div>
            <button type="submit" class="primary-btn">💾 Сохранить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    document.getElementById('edit-key-form').onsubmit = async (e) => {
        e.preventDefault();
        const data = {
            key_data: document.getElementById('key_data').value.toUpperCase(),
            comment: document.getElementById('comment').value
        };
        try {
            await api.request(`/api/keys/${key.ID}`, { method: 'PUT', body: JSON.stringify(data) });
            closeModal();
            await loadKeys();
            showMessage('✅ Изменения сохранены', false);
        } catch (err) {
            showMessage(err.message);
        }
    };

    const closeBtn = document.querySelector('#modal .close');
    if (closeBtn) {
        closeBtn.onclick = closeModal;
    }
}

async function addKey(flatsList) {
    flats = flatsList;

    if (flats.length === 0) {
        showMessage('Нет доступных квартир');
        return;
    }

    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
        <h3>🔑 Добавить ключ домофона</h3>
        <form id="add-key-form">
            <div class="form-group">
                <label for="flat_id">Квартира:</label>
                <select id="flat_id" required>
                    <option value="">Выберите квартиру</option>
                    ${flats.map(f => `<option value="${f.flat_id}">${escapeHtml(f.address)}</option>`).join('')}
                </select>
            </div>
            <div class="form-group">
                <label for="key_data">Ключ (HEX):</label>
                <input type="text" id="key_data" placeholder="Например: 1A2B3C4D" required>
                <small class="hint">HEX-строка, только цифры 0-9 и буквы A-F</small>
            </div>
            <div class="form-group">
                <label for="comment">Комментарий:</label>
                <textarea id="comment" placeholder="Описание ключа (необязательно)" rows="2"></textarea>
            </div>
            <button type="submit" class="primary-btn">➕ Добавить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    document.getElementById('add-key-form').onsubmit = async (e) => {
        e.preventDefault();
        const data = {
            flat_id: parseInt(document.getElementById('flat_id').value),
            key_data: document.getElementById('key_data').value.toUpperCase(),
            comment: document.getElementById('comment').value
        };
        try {
            await api.request('/api/keys', { method: 'POST', body: JSON.stringify(data) });
            closeModal();
            await loadKeys();
            showMessage('✅ Ключ добавлен', false);
        } catch (err) {
            showMessage(err.message);
        }
    };

    const closeBtn = document.querySelector('#modal .close');
    if (closeBtn) {
        closeBtn.onclick = closeModal;
    }
}

async function deleteKey(id) {
    try {
        await api.request(`/api/keys/${id}`, { method: 'DELETE' });
        await loadKeys();
        showMessage('✅ Ключ удалён', false);
    } catch (err) {
        showMessage('❌ Ошибка удаления: ' + err.message);
        console.error('Delete key error:', err);
    }
}

function confirmDeleteKey(id) {
    showConfirm('Вы уверены, что хотите удалить этот ключ? Это действие нельзя отменить.', () => {
        deleteKey(id);
    });
}

export { loadKeys, addKey, editKey, deleteKey, confirmDeleteKey, setFlats };