import api from './api.js';
import { showMessage } from './auth.js';
import { showConfirm, closeModal } from './modal.js';
import { showPhotoGallery } from './gallery.js';

let flats = [];
let cachedCars = [];

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Глобальная функция валидации номера в реальном времени
window.validatePlate = function(input) {
    const value = input.value.toUpperCase();
    const regex = /^[АВЕКМНОРСТУХA-Z]{0,1}\d{0,3}[АВЕКМНОРСТУХA-Z]{0,2}\d{0,3}$/;
    const hint = input.parentElement.querySelector('.hint');
    const errorSpan = input.parentElement.querySelector('.field-error');

    // Удаляем старую ошибку
    if (errorSpan) errorSpan.remove();

    if (value.length > 0 && !regex.test(value)) {
        input.style.borderColor = '#ef4444';
        if (hint) hint.style.color = '#ef4444';

        // Показываем конкретную ошибку
        const error = document.createElement('div');
        error.className = 'field-error';
        error.style.cssText = 'color: #ef4444; font-size: 12px; margin-top: 5px;';

        if (value.length === 1 && !/[АВЕКМНОРСТУХA-Z]/i.test(value)) {
            error.textContent = '⚠️ Первая буква должна быть из списка: А, В, Е, К, М, Н, О, Р, С, Т, У, Х';
        } else {
            error.textContent = '⚠️ Неверный формат. Пример: A123BC159';
        }

        input.parentElement.appendChild(error);
    } else {
        input.style.borderColor = '#e0e0e0';
        if (hint) hint.style.color = '#7f8c8d';
    }

    // Обновляем подсказку
    if (hint) {
        if (value.length === 1 && !/[АВЕКМНОРСТУХA-Z]/i.test(value)) {
            hint.textContent = 'Первая буква должна быть из списка: А,В,Е,К,М,Н,О,Р,С,Т,У,Х';
        } else {
            hint.textContent = 'Формат: буква, 3 цифры, 2 буквы, 2-3 цифры региона';
        }
    }
};

function setFlats(flatsData) {
    flats = flatsData;
}

async function loadCars() {
    try {
        const cars = await api.request('/api/cars');
        cachedCars = cars;
        const container = document.getElementById('cars-list');

        if (cars.length === 0) {
            container.innerHTML = '<p class="empty-message">📭 Нет добавленных автомобилей</p>';
            return;
        }

        container.innerHTML = cars.map(car => {
            const daysUntilExpiry = Math.ceil((new Date(car.ExpiresAt) - new Date()) / (1000 * 60 * 60 * 24));
            const showExtend = daysUntilExpiry <= 7 && daysUntilExpiry > 0;
            const mainPhoto = car.Photos?.find(p => p.IsMain) || car.Photos?.[0];
            const photoSrc = mainPhoto ? mainPhoto.PhotoData : '';
            const flatAddress = flats.find(f => f.flat_id === car.FlatID)?.address || 'Адрес не найден';

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
                <div class="address">📍 ${escapeHtml(flatAddress)}</div>
                <div class="notify-icons">
                    ${car.AutoOpen ? '<span>🚪 Автооткрытие</span>' : ''}
                    ${car.NotifyOnDetect ? '<span>📡 Обнаружение</span>' : ''}
                    ${car.NotifyOnEntry ? '<span>🚗 Въезд</span>' : ''}
                    ${car.NotifyOnExit ? '<span>🏁 Выезд</span>' : ''}
                </div>
                <div class="expiry ${new Date(car.ExpiresAt) < new Date() ? 'expired' : ''}">
                    📅 Действует до: ${new Date(car.ExpiresAt).toLocaleDateString()}
                    ${new Date(car.ExpiresAt) < new Date() ? ' (Истек)' : ` (осталось ${daysUntilExpiry} дней)`}
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
            const result = await api.request(`/api/cars/extend/${id}`, {
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

async function addCar(flatsList) {
    flats = flatsList;

    if (flats.length === 0) {
        showMessage('❌ Нет доступных квартир');
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
                <input type="text" id="plate_number" placeholder="Например: A123BC159 или О743УХ159" required oninput="validatePlate(this)">
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

        const flatId = parseInt(document.getElementById('flat_id').value);
        if (!flatId) {
            showMessage('❌ Выберите квартиру');
            return;
        }

        const plateNumber = document.getElementById('plate_number').value.trim();
        if (!plateNumber) {
            showMessage('❌ Введите номер автомобиля');
            return;
        }

        // Проверка формата перед отправкой
        const normalizedPlate = plateNumber.toUpperCase();
        const plateRegex = /^[A-ZА-Я]\d{3}[A-ZА-Я]{2}\d{2,3}$/;
        if (!plateRegex.test(normalizedPlate)) {
            showMessage('❌ Неверный формат номера. Используйте формат: A123BC159');
            return;
        }

        const data = {
            flat_id: flatId,
            plate_number: plateNumber.toUpperCase(),
            comment: document.getElementById('comment').value,
            auto_open: document.getElementById('auto_open').checked,
            notify_on_detect: document.getElementById('notify_detect').checked,
            notify_on_entry: document.getElementById('notify_entry').checked,
            notify_on_exit: document.getElementById('notify_exit').checked,
            expires_in_days: document.getElementById('expiry').value
        };

        try {
            const result = await api.request('/api/cars', { method: 'POST', body: JSON.stringify(data) });
            closeModal();
            showMessage('✅ ' + (result.message || 'Автомобиль добавлен'), false);
            await loadCars();
        } catch (err) {
            console.error('Add car error:', err);
            showMessage('❌ ' + err.message);
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
    showEditModal(car);
}

function showEditModal(car) {
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
            const result = await api.request(`/api/cars/${car.ID}`, { method: 'PUT', body: JSON.stringify(data) });
            closeModal();
            showMessage('✅ ' + (result.message || 'Изменения сохранены'), false);
            await loadCars();
        } catch (err) {
            showMessage('❌ ' + err.message);
        }
    };

    const closeBtn = document.querySelector('#modal .close');
    if (closeBtn) {
        closeBtn.onclick = closeModal;
    }
}

async function deleteCar(id) {
    try {
        const result = await api.request(`/api/cars/${id}`, { method: 'DELETE' });
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

export { loadCars, addCar, editCar, deleteCar, confirmDeleteCar, extendCar, setFlats };