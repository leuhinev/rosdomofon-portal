import api from './api.js';
import { showMessage } from './auth.js';
import { showConfirm, closeModal } from './modal.js';
import { showPhotoGallery } from './gallery.js';

let addresses = [];
let cachedCars = [];

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function setAddresses(addressesData) {
    addresses = addressesData;
}

async function loadCars() {
    try {
        const cars = await api.request('./api/cars');
        cachedCars = cars;
        const container = document.getElementById('cars-list');
        if (cars.length === 0) {
            container.innerHTML = '<p class="empty-message">📭 Нет добавленных автомобилей</p>';
            return;
        }
        container.innerHTML = cars.map(car => {
            const expiryDate = new Date(car.ExpiresAt);
            const now = new Date();
            const daysUntilExpiry = Math.ceil((expiryDate - now) / (1000 * 60 * 60 * 24));
            const showExtend = daysUntilExpiry <= 7;
            const mainPhoto = car.Photos?.find(p => p.IsMain) || car.Photos?.[0];
            const photoSrc = mainPhoto ? mainPhoto.PhotoData : '';
            // Используем car.address (с маленькой буквы, как приходит с сервера)
            const address = car.address || 'Адрес не найден';

            const expiryDateStr = expiryDate.toLocaleDateString('ru-RU');
            const expiryTimeStr = expiryDate.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });

            return `
            <div class="car-item" data-id="${car.ID}">
                <div class="car-header">
                    <div class="car-info">
                        ${photoSrc ?
                `<img src="${photoSrc}" class="car-thumbnail" onclick="window.showPhotoGallery('${photoSrc.replace(/'/g, "\\'")}')" alt="фото">` :
                '<div class="car-thumbnail placeholder">📷</div>'}
                        <span class="plate-number">${escapeHtml(car.PlateNumber)}</span>
                    </div>
                    <div class="actions">
                        ${showExtend ? `<button class="extend-btn" onclick="window.extendCar(${car.ID})">🔄 Продлить</button>` : ''}
                        <button class="edit-btn" onclick="window.editCar(${car.ID})">✏️ Редактировать</button>
                        <button class="delete-btn" onclick="window.confirmDeleteCar(${car.ID})">🗑️ Удалить</button>
                    </div>
                </div>
                ${car.Comment ? `<div class="comment">💬 ${escapeHtml(car.Comment)}</div>` : ''}
                <div class="address">📍 ${escapeHtml(address)}</div>
                <div class="notify-icons">
                    ${car.AutoOpen ? '<span>🚪 Автооткрытие</span>' : ''}
                    ${car.NotifyOnDetect ? '<span>📡 Обнаружение</span>' : ''}
                    ${car.NotifyOnEntry ? '<span>🚗 Въезд</span>' : ''}
                    ${car.NotifyOnExit ? '<span>🏁 Выезд</span>' : ''}
                </div>
                <div class="expiry ${expiryDate < now ? 'expired' : ''}">
                    📅 Действует до: ${expiryDateStr} ${expiryTimeStr}
                    ${expiryDate < now ? ' (Истек)' : ` (осталось ${daysUntilExpiry} дней)`}
                </div>
            </div>
        `}).join('');
    } catch (err) {
        console.error('Error loading cars:', err);
        showMessage('❌ ' + err.message);
        document.getElementById('cars-list').innerHTML = '<p class="error-message">❌ Ошибка загрузки автомобилей</p>';
    }
}

async function extendCar(id) {
    const days = await new Promise((resolve) => {
        const modalBody = document.getElementById('modal-body');
        modalBody.innerHTML = `
            <h3>🔄 Продлить срок действия</h3>
            <form id="extend-form">
                <div class="form-group">
                    <label for="extend_days">Выберите период:</label>
                    <select id="extend_days" required>
                        <option value="1">1 день</option>
                        <option value="3">3 дня</option>
                        <option value="7">1 неделя</option>
                        <option value="30">1 месяц</option>
                        <option value="90">3 месяца</option>
                        <option value="180">6 месяцев</option>
                        <option value="365">1 год</option>
                    </select>
                </div>
                <button type="submit" class="primary-btn">Продлить</button>
            </form>
        `;
        document.getElementById('modal').style.display = 'block';

        document.getElementById('extend-form').onsubmit = async (e) => {
            e.preventDefault();
            const days = parseInt(document.getElementById('extend_days').value);
            closeModal();
            resolve(days);
        };

        const closeBtn = document.querySelector('#modal .close');
        if (closeBtn) {
            closeBtn.onclick = () => {
                closeModal();
                resolve(null);
            };
        }
    });

    if (days) {
        try {
            const result = await api.request(`./api/cars/extend/${id}`, {
                method: 'POST',
                body: JSON.stringify({ additional_days: days })
            });
            showMessage('✅ ' + (result.message || 'Срок действия продлён'), false);
            await loadCars();
        } catch (err) {
            showMessage('❌ ' + err.message);
        }
    }
}

async function addCar(addressesList) {
    addresses = addressesList;

    if (addresses.length === 0) {
        showMessage('❌ Нет доступных адресов');
        return;
    }

    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
        <h3>🚗 Добавить автомобиль</h3>
        <form id="add-car-form">
            <div class="form-group">
                <label for="address_id">Адрес:</label>
                <select id="address_id" required>
                    <option value="">Выберите адрес</option>
                    ${addresses.map(a => `<option value="${a.address_id}">${escapeHtml(a.address)}</option>`).join('')}
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
                    <option value="day">1 день (текущие сутки до 23:59)</option>
                    <option value="week">1 неделя</option>
                    <option value="month">1 месяц</option>
                    <option value="3months">3 месяца</option>
                    <option value="6months">6 месяцев</option>
                    <option value="year">1 год</option>
                </select>
            </div>
            <div id="form-error" style="color:#dc2626; font-size:14px; margin-bottom:15px; padding:10px; background:#fee2e2; border-radius:8px; display:none;"></div>
            <button type="submit" class="primary-btn">➕ Добавить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    const form = document.getElementById('add-car-form');
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

        const plateNumber = document.getElementById('plate_number').value.trim();
        if (!plateNumber) {
            errorDiv.textContent = '❌ Введите номер автомобиля';
            errorDiv.style.display = 'block';
            return;
        }

        // Простая проверка формата перед отправкой
        const plateRegex = /^[АВЕКМНОРСТУХA-Z]\d{3}[АВЕКМНОРСТУХA-Z]{2}\d{2,3}$/i;
        if (!plateRegex.test(plateNumber)) {
            errorDiv.textContent = '❌ Неверный формат номера. Пример: A123BC159 или О743УХ159';
            errorDiv.style.display = 'block';
            return;
        }

        const data = {
            address_id: addressId,
            plate_number: plateNumber.toUpperCase(),
            comment: document.getElementById('comment').value,
            auto_open: document.getElementById('auto_open').checked,
            notify_on_detect: document.getElementById('notify_detect').checked,
            notify_on_entry: document.getElementById('notify_entry').checked,
            notify_on_exit: document.getElementById('notify_exit').checked,
            expires_in_days: document.getElementById('expiry').value
        };

        const submitBtn = form.querySelector('button[type="submit"]');
        const originalText = submitBtn.textContent;
        submitBtn.textContent = '⏳ Добавление...';
        submitBtn.disabled = true;

        try {
            const result = await api.request('./api/cars', { method: 'POST', body: JSON.stringify(data) });
            closeModal();
            showMessage('✅ ' + (result.message || 'Автомобиль добавлен'), false);
            await loadCars();
        } catch (err) {
            console.error('Add car error:', err);
            errorDiv.textContent = '❌ ' + err.message;
            errorDiv.style.display = 'block';
            const plateInput = document.getElementById('plate_number');
            plateInput.style.borderColor = '#ef4444';
            setTimeout(() => {
                plateInput.style.borderColor = '#e0e0e0';
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

async function editCar(id) {
    const car = cachedCars.find(c => c.ID === id);
    if (!car) {
        showMessage('❌ Автомобиль не найден');
        return;
    }

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
            <div id="form-error" style="color:#dc2626; font-size:14px; margin-bottom:15px; padding:10px; background:#fee2e2; border-radius:8px; display:none;"></div>
            <button type="submit" class="primary-btn">💾 Сохранить</button>
        </form>
    `;
    document.getElementById('modal').style.display = 'block';

    const form = document.getElementById('edit-car-form');
    const errorDiv = document.getElementById('form-error');

    form.onsubmit = async (e) => {
        e.preventDefault();

        errorDiv.style.display = 'none';
        errorDiv.textContent = '';

        const data = {
            comment: document.getElementById('comment').value,
            auto_open: document.getElementById('auto_open').checked,
            notify_on_detect: document.getElementById('notify_detect').checked,
            notify_on_entry: document.getElementById('notify_entry').checked,
            notify_on_exit: document.getElementById('notify_exit').checked
        };

        const submitBtn = form.querySelector('button[type="submit"]');
        const originalText = submitBtn.textContent;
        submitBtn.textContent = '⏳ Сохранение...';
        submitBtn.disabled = true;

        try {
            const result = await api.request(`./api/cars/${car.ID}`, { method: 'PUT', body: JSON.stringify(data) });
            closeModal();
            showMessage('✅ ' + (result.message || 'Изменения сохранены'), false);
            await loadCars();
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

async function deleteCar(id) {
    try {
        const result = await api.request(`./api/cars/${id}`, { method: 'DELETE' });
        showMessage('✅ ' + (result.message || 'Автомобиль удалён'), false);
        await loadCars();
    } catch (err) {
        showMessage('❌ ' + err.message);
    }
}

function confirmDeleteCar(id) {
    showConfirm('Вы уверены, что хотите удалить этот автомобиль? Это действие нельзя отменить.', () => {
        deleteCar(id);
    });
}

export { loadCars, addCar, editCar, deleteCar, confirmDeleteCar, extendCar, setAddresses };