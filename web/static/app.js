let token = null;
let flats = [];
let pendingDelete = null;
let pendingDeleteType = null;

const api = {
    async request(endpoint, options = {}) {
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };
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

function showMessage(text, isError = true) {
    const msgDiv = document.getElementById('message');
    msgDiv.textContent = text;
    msgDiv.className = isError ? 'error' : 'success';
    setTimeout(() => {
        msgDiv.textContent = '';
        msgDiv.className = '';
    }, 5000);
}

function showConfirm(message, onConfirm) {
    const confirmModal = document.getElementById('confirm-modal');
    const confirmMessage = document.getElementById('confirm-message');
    const confirmYes = document.getElementById('confirm-yes');
    const confirmNo = document.getElementById('confirm-no');
    const closeConfirm = document.querySelector('.close-confirm');

    confirmMessage.textContent = message;
    confirmModal.style.display = 'block';

    const handleConfirm = () => {
        confirmModal.style.display = 'none';
        confirmYes.removeEventListener('click', handleConfirm);
        confirmNo.removeEventListener('click', handleCancel);
        closeConfirm.removeEventListener('click', handleCancel);
        onConfirm();
    };

    const handleCancel = () => {
        confirmModal.style.display = 'none';
        confirmYes.removeEventListener('click', handleConfirm);
        confirmNo.removeEventListener('click', handleCancel);
        closeConfirm.removeEventListener('click', handleCancel);
    };

    confirmYes.addEventListener('click', handleConfirm);
    confirmNo.addEventListener('click', handleCancel);
    closeConfirm.addEventListener('click', handleCancel);

    window.onclick = (e) => {
        if (e.target === confirmModal) {
            handleCancel();
        }
    };
}

function logout() {
    token = null;
    localStorage.removeItem('token');
    document.getElementById('auth-screen').classList.add('active');
    document.getElementById('portal-screen').classList.remove('active');
    document.getElementById('phone').value = '';
    document.getElementById('code').value = '';
    document.getElementById('code-section').style.display = 'none';
    const submitBtn = document.getElementById('submit-btn');
    submitBtn.innerHTML = 'Получить код';
    submitBtn.disabled = false;
}

async function loadFlats() {
    const data = await api.request('/api/user/flats');
    flats = data;
    return flats;
}

async function loadCars() {
    try {
        const cars = await api.request('/api/cars');
        const container = document.getElementById('cars-list');
        if (cars.length === 0) {
            container.innerHTML = '<p class="empty-message">📭 Нет добавленных автомобилей</p>';
            return;
        }
        container.innerHTML = cars.map(car => `
            <div class="car-item" data-id="${car.ID}">
                <div class="car-header">
                    <span class="plate-number">${escapeHtml(car.PlateNumber)}</span>
                    <div class="actions">
                        <button class="edit-btn" onclick="editCar(${car.ID})">✏️ Редактировать</button>
                        <button class="delete-btn" onclick="confirmDeleteCar(${car.ID})">🗑️ Удалить</button>
                    </div>
                </div>
                ${car.Comment ? `<div class="comment">💬 ${escapeHtml(car.Comment)}</div>` : ''}
                <div class="address">📍 ${flats.find(f => f.flat_id === car.FlatID)?.address || 'Адрес не найден'}</div>
                <div class="notify-icons">
                    ${car.AutoOpen ? '<span>🚪 Автооткрытие</span>' : ''}
                    ${car.NotifyOnDetect ? '<span>📡 Обнаружение</span>' : ''}
                    ${car.NotifyOnEntry ? '<span>🚗 Въезд</span>' : ''}
                    ${car.NotifyOnExit ? '<span>🏁 Выезд</span>' : ''}
                </div>
                <div class="expiry ${new Date(car.ExpiresAt) < new Date() ? 'expired' : ''}">
                    📅 Действует до: ${new Date(car.ExpiresAt).toLocaleDateString()}
                    ${new Date(car.ExpiresAt) < new Date() ? ' (Истек)' : ''}
                </div>
            </div>
        `).join('');
    } catch (err) {
        console.error('Error loading cars:', err);
        document.getElementById('cars-list').innerHTML = '<p class="error-message">❌ Ошибка загрузки автомобилей</p>';
    }
}

async function loadKeys() {
    try {
        const keys = await api.request('/api/keys');
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
                        <button class="edit-btn" onclick="editKey(${key.ID})">✏️ Редактировать</button>
                        <button class="delete-btn" onclick="confirmDeleteKey(${key.ID})">🗑️ Удалить</button>
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

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function maskKey(key) {
    if (!key) return '';
    if (key.length <= 8) return '****' + key.slice(-4);
    return key.slice(0, 6) + '...' + key.slice(-4);
}

function confirmDeleteCar(id) {
    showConfirm('Вы уверены, что хотите удалить этот автомобиль? Это действие нельзя отменить.', () => {
        deleteCar(id);
    });
}

function confirmDeleteKey(id) {
    showConfirm('Вы уверены, что хотите удалить этот ключ? Это действие нельзя отменить.', () => {
        deleteKey(id);
    });
}

async function deleteCar(id) {
    try {
        await api.request(`/api/cars/${id}`, { method: 'DELETE' });
        await loadCars();
        showMessage('✅ Автомобиль удалён', false);
    } catch (err) {
        showMessage('❌ Ошибка удаления: ' + err.message);
        console.error('Delete car error:', err);
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

async function addCar() {
    if (flats.length === 0) {
        showMessage('Нет доступных квартир');
        return;
    }

    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
        <h3>🚗 Добавить автомобиль</h3>
        <form id="add-car-form">
            <div class="form-group">
                <label for="flat_id">Квартира:</label>
                <select id="flat_id" required>
                    <option value="">Выберите квартиру</option>
                    ${flats.map(f => `<option value="${f.flat_id}">${escapeHtml(f.address)}</option>`).join('')}
                </select>
            </div>
            <div class="form-group">
                <label for="plate_number">Номер автомобиля:</label>
                <input type="text" id="plate_number" placeholder="Например: A123BC159 или О743УХ159" required>
                <small class="hint">Формат: буква, 3 цифры, 2 буквы, 2-3 цифры региона</small>
            </div>
            <div class="form-group">
                <label for="comment">Комментарий:</label>
                <textarea id="comment" placeholder="Кому принадлежит (необязательно)" rows="2"></textarea>
            </div>
            <div class="form-group">
                <label>⚙️ Настройки автоматики:</label>
                <div class="checkbox-group">
                    <label>
                        <input type="checkbox" id="auto_open"> 
                        🚪 Открывать при обнаружении
                    </label>
                </div>
            </div>
            <div class="form-group">
                <label>🔔 Настройки уведомлений:</label>
                <div class="checkbox-group-horizontal">
                    <label>
                        <input type="checkbox" id="notify_detect"> 
                        📡 Обнаружение
                    </label>
                    <label>
                        <input type="checkbox" id="notify_entry"> 
                        🚗 Въезд
                    </label>
                    <label>
                        <input type="checkbox" id="notify_exit"> 
                        🏁 Выезд
                    </label>
                </div>
            </div>
            <div class="form-group">
                <label for="expiry">⏰ Срок действия:</label>
                <select id="expiry" required>
                    <option value="day">1 день</option>
                    <option value="week">1 неделя</option>
                    <option value="month">1 месяц</option>
                    <option value="3months">3 месяца</option>
                    <option value="6months">6 месяцев</option>
                    <option value="year">1 год</option>
                </select>
            </div>
            <button type="submit" class="primary-btn">➕ Добавить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    document.getElementById('add-car-form').onsubmit = async (e) => {
        e.preventDefault();
        const data = {
            flat_id: parseInt(document.getElementById('flat_id').value),
            plate_number: document.getElementById('plate_number').value.toUpperCase(),
            comment: document.getElementById('comment').value,
            auto_open: document.getElementById('auto_open').checked,
            notify_on_detect: document.getElementById('notify_detect').checked,
            notify_on_entry: document.getElementById('notify_entry').checked,
            notify_on_exit: document.getElementById('notify_exit').checked,
            expires_in_days: document.getElementById('expiry').value
        };
        try {
            await api.request('/api/cars', { method: 'POST', body: JSON.stringify(data) });
            closeModal();
            loadCars();
            showMessage('✅ Автомобиль добавлен', false);
        } catch (err) {
            showMessage(err.message);
        }
    };
}

async function editCar(id) {
    const cars = await api.request('/api/cars');
    const car = cars.find(c => c.ID === id);
    if (!car) return;

    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
        <h3>✏️ Редактировать автомобиль</h3>
        <form id="edit-car-form">
            <div class="form-group">
                <label>Номер автомобиля:</label>
                <p class="static-info">${escapeHtml(car.PlateNumber)}</p>
                <small class="hint">Номер нельзя изменить</small>
            </div>
            <div class="form-group">
                <label for="comment">Комментарий:</label>
                <textarea id="comment" rows="2">${escapeHtml(car.Comment || '')}</textarea>
            </div>
            <div class="form-group">
                <label>⚙️ Настройки автоматики:</label>
                <div class="checkbox-group">
                    <label>
                        <input type="checkbox" id="auto_open" ${car.AutoOpen ? 'checked' : ''}> 
                        🚪 Открывать при обнаружении
                    </label>
                </div>
            </div>
            <div class="form-group">
                <label>🔔 Настройки уведомлений:</label>
                <div class="checkbox-group-horizontal">
                    <label>
                        <input type="checkbox" id="notify_detect" ${car.NotifyOnDetect ? 'checked' : ''}> 
                        📡 Обнаружение
                    </label>
                    <label>
                        <input type="checkbox" id="notify_entry" ${car.NotifyOnEntry ? 'checked' : ''}> 
                        🚗 Въезд
                    </label>
                    <label>
                        <input type="checkbox" id="notify_exit" ${car.NotifyOnExit ? 'checked' : ''}> 
                        🏁 Выезд
                    </label>
                </div>
            </div>
            <button type="submit" class="primary-btn">💾 Сохранить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    document.getElementById('edit-car-form').onsubmit = async (e) => {
        e.preventDefault();
        const data = {
            comment: document.getElementById('comment').value,
            auto_open: document.getElementById('auto_open').checked,
            notify_on_detect: document.getElementById('notify_detect').checked,
            notify_on_entry: document.getElementById('notify_entry').checked,
            notify_on_exit: document.getElementById('notify_exit').checked
        };
        try {
            await api.request(`/api/cars/${id}`, { method: 'PUT', body: JSON.stringify(data) });
            closeModal();
            loadCars();
            showMessage('✅ Изменения сохранены', false);
        } catch (err) {
            showMessage(err.message);
        }
    };
}

async function addKey() {
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
            loadKeys();
            showMessage('✅ Ключ добавлен', false);
        } catch (err) {
            showMessage(err.message);
        }
    };
}

async function editKey(id) {
    const keys = await api.request('/api/keys');
    const key = keys.find(k => k.ID === id);
    if (!key) return;

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
            await api.request(`/api/keys/${id}`, { method: 'PUT', body: JSON.stringify(data) });
            closeModal();
            loadKeys();
            showMessage('✅ Изменения сохранены', false);
        } catch (err) {
            showMessage(err.message);
        }
    };
}

function closeModal() {
    document.getElementById('modal').style.display = 'none';
}

document.addEventListener('DOMContentLoaded', () => {
    const savedToken = localStorage.getItem('token');
    if (savedToken) {
        token = savedToken;
        initPortal();
    }

    const phoneInput = document.getElementById('phone');
    const codeInput = document.getElementById('code');
    const submitBtn = document.getElementById('submit-btn');
    const codeSection = document.getElementById('code-section');

    let waitingForCode = false;

    document.getElementById('auth-form').onsubmit = async (e) => {
        e.preventDefault();

        const phone = phoneInput.value.trim();

        if (!waitingForCode) {
            if (!phone.match(/^\d{10}$/)) {
                showMessage('Введите 10 цифр номера телефона (без +7)');
                phoneInput.focus();
                return;
            }

            const fullPhone = '+7' + phone;

            submitBtn.disabled = true;
            const originalText = submitBtn.innerHTML;
            submitBtn.innerHTML = '<span class="loading"></span> Отправка...';

            try {
                const response = await fetch('/api/auth/send-code', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ phone: fullPhone })
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error);

                codeSection.style.display = 'block';
                submitBtn.innerHTML = 'Войти';
                submitBtn.disabled = false;
                waitingForCode = true;
                codeInput.focus();

                showMessage('Код отправлен в push-уведомление', false);
            } catch (err) {
                showMessage(err.message);
                submitBtn.innerHTML = originalText;
                submitBtn.disabled = false;
                codeSection.style.display = 'none';
                waitingForCode = false;
            }
        } else {
            const code = codeInput.value.trim();
            if (!code.match(/^\d{6}$/)) {
                showMessage('Введите 6-значный код из push-уведомления');
                codeInput.focus();
                return;
            }

            const fullPhone = '+7' + phone;

            submitBtn.disabled = true;
            const originalText = submitBtn.innerHTML;
            submitBtn.innerHTML = '<span class="loading"></span> Проверка...';

            try {
                const response = await fetch('/api/auth/verify', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ phone: fullPhone, code })
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error);

                token = data.access_token;
                localStorage.setItem('token', token);
                await initPortal();
            } catch (err) {
                showMessage(err.message);
                codeInput.value = '';
                codeInput.focus();
                submitBtn.innerHTML = 'Войти';
                submitBtn.disabled = false;
            }
        }
    };

    document.getElementById('logout-btn').onclick = logout;
    document.querySelector('.close').onclick = closeModal;

    window.onclick = (e) => {
        const modal = document.getElementById('modal');
        const confirmModal = document.getElementById('confirm-modal');
        if (e.target === modal) closeModal();
    };

    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.onclick = () => {
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
            btn.classList.add('active');
            document.getElementById(`${btn.dataset.tab}-tab`).classList.add('active');
            if (btn.dataset.tab === 'cars') loadCars();
            if (btn.dataset.tab === 'keys') loadKeys();
        };
    });

    const addCarBtn = document.getElementById('add-car-btn');
    const addKeyBtn = document.getElementById('add-key-btn');
    if (addCarBtn) addCarBtn.onclick = addCar;
    if (addKeyBtn) addKeyBtn.onclick = addKey;

    window.addCar = addCar;
    window.editCar = editCar;
    window.deleteCar = deleteCar;
    window.confirmDeleteCar = confirmDeleteCar;
    window.confirmDeleteKey = confirmDeleteKey;
    window.addKey = addKey;
    window.editKey = editKey;
    window.deleteKey = deleteKey;
    window.closeModal = closeModal;
});

async function initPortal() {
    try {
        await loadFlats();
        await loadCars();
        document.getElementById('auth-screen').classList.remove('active');
        document.getElementById('portal-screen').classList.add('active');
        showMessage('Добро пожаловать!', false);
    } catch (err) {
        console.error('Init error:', err);
        showMessage('Ошибка загрузки данных. Попробуйте перезагрузить страницу.');
        logout();
    }
}