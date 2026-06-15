let token = null;

function showMessage(text, isError = true) {
    const msgDiv = document.getElementById('message');
    msgDiv.textContent = text;
    msgDiv.className = isError ? 'error' : 'success';
    setTimeout(() => {
        msgDiv.textContent = '';
        msgDiv.className = '';
    }, 5000);
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

async function refreshTokenIfNeeded() {
    const savedToken = localStorage.getItem('token');
    if (!savedToken) return false;

    try {
        const payload = JSON.parse(atob(savedToken.split('.')[1]));
        const expTime = payload.exp * 1000;
        const now = Date.now();
        const daysUntilExpiry = (expTime - now) / (1000 * 60 * 60 * 24);

        if (daysUntilExpiry < 7) {
            const response = await fetch('/api/auth/refresh', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ token: savedToken })
            });
            const data = await response.json();
            if (response.ok) {
                token = data.access_token;
                localStorage.setItem('token', token);
                return true;
            }
        } else {
            token = savedToken;
            return true;
        }
    } catch (err) {
        console.error('Refresh failed:', err);
    }
    return false;
}

function getToken() {
    return token;
}

function setToken(newToken) {
    token = newToken;
    localStorage.setItem('token', token);
}

export { getToken, setToken, logout, showMessage, refreshTokenIfNeeded };