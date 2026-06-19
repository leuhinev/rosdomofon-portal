import api from './api.js';
import { showMessage } from './auth.js';
import { showConfirm, closeModal } from './modal.js';

let addresses = [];
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

function setAddresses(addressesData) {
    addresses = addressesData;
}

async function loadKeys() {
    try {
        const keys = await api.request('./api/keys');
        cachedKeys = keys;
        const container = document.getElementById('keys-list');
        if (keys.length === 0) {
            container.innerHTML = '<p class="empty-message">🔐 Нет добавленных ключей</p>';
            return;
        }
        container.innerHTML = keys.map(key => {
            // Используем key.address (с маленькой буквы, как приходит с сервера)
            const address = key.address || 'Адрес не найден';
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
                <div class="address">📍 ${escapeHtml(address)}</div>
            </div>
        `}).join('');
    } catch (err) {
        console.error('Error loading keys:', err);
        document.getElementById('keys-list').innerHTML = '<p class="error-message">❌ Ошибка загрузки ключей</p>';
    }
}

async function addKey(addressesList) {
    addresses = addressesList;

    if (addresses.length === 0) {
        showMessage('❌ Нет доступных адресов');
        return;
    }

    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
        <h3>🔑 Добавить ключ домофона</h3>
        <form id="add-key-form">
            <div class="form-group">
                <label for="address_id">Адрес:</label>
                <select id="address_id" required>
                    <option value="">Выберите адрес</option>
                    ${addresses.map(a => `<option value="${a.address_id}">${escapeHtml(a.address)}</option>`).join('')}
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

        errorDiv.style.display = 'none';
        errorDiv.textContent = '';

        const addressId = parseInt(document.getElementById('address_id').value);
        if (!addressId) {
            errorDiv.textContent = '❌ Выберите адрес';
            errorDiv.style.display = 'block';
            return;
        }

        const keyData = document.getElementById('key_data').value.trim();
        if (!keyData) {
            errorDiv.textContent = '❌ Введите ключ';
            errorDiv.style.display = 'block';
            return;
        }

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
            address_id: addressId,
            key_data: keyData.toUpperCase(),
            comment: document.getElementById('comment').value
        };

        const submitBtn = form.querySelector('button[type="submit"]');
        const originalText = submitBtn.textContent;
        submitBtn.textContent = '⏳ Добавление...';
        submitBtn.disabled = true;

        try {
            const result = await api.request('./api/keys', { method: 'POST', body: JSON.stringify(data) });
            closeModal();
            showMessage('✅ ' + (result.message || 'Ключ добавлен'), false);
            await loadKeys();
        } catch (err) {
            errorDiv.textContent = '❌ ' + err.message;
            errorDiv.style.display = 'block';
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
            <div id="form-error" style="color:#dc2626; font-size:14px; margin-bottom:15px; padding:10px; background:#fee2e2; border-radius:8px; display:none;"></div>
            <button type="submit" class="primary-btn">💾 Сохранить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    const form = document.getElementById('edit-key-form');
    const errorDiv = document.getElementById('form-error');

    form.onsubmit = async (e) => {
        e.preventDefault();

        errorDiv.style.display = 'none';
        errorDiv.textContent = '';

        const data = {
            key_data: document.getElementById('key_data').value.toUpperCase(),
            comment: document.getElementById('comment').value
        };

        const submitBtn = form.querySelector('button[type="submit"]');
        const originalText = submitBtn.textContent;
        submitBtn.textContent = '⏳ Сохранение...';
        submitBtn.disabled = true;

        try {
            const result = await api.request(`./api/keys/${key.ID}`, { method: 'PUT', body: JSON.stringify(data) });
            closeModal();
            showMessage('✅ ' + (result.message || 'Изменения сохранены'), false);
            await loadKeys();
        } catch (err) {
            errorDiv.textContent = '❌ ' + err.message;
            errorDiv.style.display = 'block';
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

async function deleteKey(id) {
    try {
        const result = await api.request(`./api/keys/${id}`, { method: 'DELETE' });
        showMessage('✅ ' + (result.message || 'Ключ удалён'), false);
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

export { loadKeys, addKey, editKey, deleteKey, confirmDeleteKey, setAddresses };