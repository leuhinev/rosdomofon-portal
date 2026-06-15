import { getToken, setToken, logout, refreshTokenIfNeeded, showMessage } from './auth.js';
import api from './api.js';
import { loadCars, addCar, editCar, deleteCar, confirmDeleteCar, extendCar, setFlats as setCarsFlats } from './cars.js';
import { loadKeys, addKey, editKey, deleteKey, confirmDeleteKey, setFlats as setKeysFlats } from './keys.js';
import { showPhotoGallery } from './gallery.js';
import { closeModal } from './modal.js';

let flats = [];

function hideLoading() {
    const overlay = document.getElementById('loading-overlay');
    if (overlay) {
        overlay.classList.add('hide');
        setTimeout(() => {
            overlay.style.display = 'none';
        }, 300);
    }
}

async function loadFlats() {
    const data = await api.request('/api/user/flats');
    flats = data;
    setCarsFlats(flats);
    setKeysFlats(flats);
    return flats;
}

async function initPortal() {
    try {
        const overlay = document.getElementById('loading-overlay');
        if (overlay) {
            overlay.style.display = 'flex';
            overlay.classList.remove('hide');
        }

        console.log('Initializing portal...');
        await loadFlats();
        await loadCars();
        await loadKeys();

        document.getElementById('auth-screen').classList.remove('active');
        document.getElementById('portal-screen').classList.add('active');
        showMessage('Добро пожаловать!', false);
        console.log('Portal initialized successfully');
    } catch (err) {
        console.error('Init error:', err);
        showMessage('Ошибка загрузки данных. Попробуйте перезагрузить страницу.');
        logout();
    } finally {
        hideLoading();
    }
}

document.addEventListener('DOMContentLoaded', async () => {
    console.log('DOM loaded, checking token...');
    const isLoggedIn = await refreshTokenIfNeeded();

    if (isLoggedIn) {
        console.log('User is logged in, initializing portal...');
        await initPortal();
    } else {
        console.log('User is not logged in, showing auth screen');
        hideLoading();
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

                setToken(data.access_token);
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

    const closeButtons = document.querySelectorAll('.close, .close-confirm');
    closeButtons.forEach(btn => {
        btn.onclick = () => {
            document.getElementById('modal').style.display = 'none';
            document.getElementById('confirm-modal').style.display = 'none';
        };
    });

    window.onclick = (e) => {
        const modal = document.getElementById('modal');
        const confirmModal = document.getElementById('confirm-modal');
        if (e.target === modal) closeModal();
        if (e.target === confirmModal) {
            confirmModal.style.display = 'none';
        }
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
    if (addCarBtn) addCarBtn.onclick = () => addCar(flats);
    if (addKeyBtn) addKeyBtn.onclick = () => addKey(flats);

    window.addCar = () => addCar(flats);
    window.editCar = editCar;
    window.deleteCar = deleteCar;
    window.confirmDeleteCar = confirmDeleteCar;
    window.extendCar = extendCar;
    window.addKey = () => addKey(flats);
    window.editKey = editKey;
    window.deleteKey = deleteKey;
    window.confirmDeleteKey = confirmDeleteKey;
    window.showPhotoGallery = showPhotoGallery;
    window.closeModal = closeModal;
});