import api from './api.js';
import { showMessage } from './auth.js';
import { showConfirm, closeModal } from './modal.js';

let flats = [];
let cachedKeys = [];

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
    console.log('keys.js: setFlats received', flatsData?.length, 'flats');
    flats = flatsData;
}

async function loadKeys() {
    console.log('keys.js: loadKeys started, flats length:', flats.length);
    try {
        const keys = await api.request('/api/keys');
        console.log('keys.js: received', keys.length, 'keys');
        cachedKeys = keys;
        const container = document.getElementById('keys-list');

        if (!container) {
            console.error('keys.js: keys-list element not found');
            return;
        }

        if (keys.length === 0) {
            container.innerHTML = '<p class="empty-message">🔐 Нет добавленных ключей</p>';
            return;
        }

        container.innerHTML = keys.map(key => {
            const flatInfo = flats.find(f => f.flat_id === key.FlatID);
            const flatAddress = flatInfo ? flatInfo.address : 'Адрес не найден (ID: ' + key.FlatID + ')';

            return `
            <div class="key-item" data-id="${key.ID}">
                <div class="key-header">
                    <span class="key-data">🔑 ${maskKey(key.KeyData)}</span>
                    <div class="actions">
                        <button class="edit-btn" onclick="window.editKey(${key.ID})">✏️ Редактировать</button>
                        <button class="delete-btn" onclick="window.confirmDeleteKey(${key.ID})">🗑️ Удалить</button>
                    </div>
                </div>
                ${key.Comment ? `<div class="comment">💬 ${escapeHtml(key.Comment)}</div>` : ''}
                <div class="address">📍 ${escapeHtml(flatAddress)}</div>
            </div>
        `}).join('');
        console.log('keys.js: rendering complete');
    } catch (err) {
        console.error('keys.js: loadKeys error:', err);
        const container = document.getElementById('keys-list');
        if (container) {
            container.innerHTML = '<p class="error-message">❌ Ошибка загрузки ключей: ' + err.message + '</p>';
        }
        throw err;
    }
}

async function addKey(flatsList) {
    flats = flatsList;

    if (flats.length === 0) {
        showMessage('❌ Нет доступных квартир');
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
            <div id="form-error" style="color:#dc2626; font-size:14px; margin-bottom:15px; padding:10px; background:#fee2e2; border-radius:8px; display:none;"></div>
            <button type="submit" class="primary-btn">➕ Добавить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    const form = document.getElementById('add-key-form');
    const errorDiv = document.getElementById('form-error');

    form.onsubmit = async (e) => {
        e.preventDefault();

        // Скрываем предыдущую ошибку
        errorDiv.style.display = 'none';
        errorDiv.textContent = '';

        const flatId = parseInt(document.getElementById('flat_id').value);
        if (!flatId) {
            errorDiv.textContent = '❌ Выберите квартиру';
            errorDiv.style.display = 'block';
            return;
        }

        const keyData = document.getElementById('key_data').value.trim();
        if (!keyData) {
            errorDiv.textContent = '❌ Введите ключ';
            errorDiv.style.display = 'block';
            return;
        }

        // Проверка HEX формата
        const hexRegex = /^[0-9A-Fa-f]+$/;
        if (!hexRegex.test(keyData)) {
            errorDiv.textContent = '❌ Неверный формат ключа. Используйте только цифры 0-9 и буквы A-F';
            errorDiv.style.display = 'block';
            return;
        }

        if (keyData.length > 32) {
            errorDiv.textContent = '❌ Ключ слишком длинный. Максимум 32 символа';
            errorDiv.style.display = 'block';
            return;
        }

        const data = {
            flat_id: flatId,
            key_data: keyData.toUpperCase(),
            comment: document.getElementById('comment').value
        };

        const submitBtn = form.querySelector('button[type="submit"]');
        const originalText = submitBtn.textContent;
        submitBtn.textContent = '⏳ Добавление...';
        submitBtn.disabled = true;

        try {
            const result = await api.request('/api/keys', { method: 'POST', body: JSON.stringify(data) });
            closeModal();
            showMessage('✅ ' + (result.message || 'Ключ добавлен'), false);
            await loadKeys();
        } catch (err) {
            errorDiv.textContent = '❌ ' + err.message;
            errorDiv.style.display = 'block';
            // Подсвечиваем поле с ключом
            const keyInput = document.getElementById('key_data');
            keyInput.style.borderColor = '#ef4444';
            setTimeout(() => {
                keyInput.style.borderColor = '#e0e0e0';
            }, 3000);
        } finally {
            submitBtn.textContent = originalText;
            submitBtn.disabled = false;
        }
    };

    const closeBtn = document.querySelector('#modal .close');
    if (closeBtn) {
        closeBtn.onclick = closeModal;
    }
}

async function editKey(id) {
    const key = cachedKeys.find(k => k.ID === id);
    if (!key) {
        showMessage('❌ Ключ не найден');
        return;
    }

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
            showMessage('✅ Изменения сохранены', false);
            await loadKeys();
        } catch (err) {
            showMessage('❌ ' + err.message);
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
        showMessage('✅ Ключ удалён', false);
        await loadKeys();
    } catch (err) {
        showMessage('❌ ' + err.message);
    }
}

function confirmDeleteKey(id) {
    showConfirm('Вы уверены, что хотите удалить этот ключ? Это действие нельзя отменить.', () => {
        deleteKey(id);
    });
}

export { loadKeys, addKey, editKey, deleteKey, confirmDeleteKey, setFlats };