let token = null;
let isWebViewAuth = false;

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
    isWebViewAuth = false;
    localStorage.removeItem('token');
    localStorage.removeItem('isWebViewAuth');
    document.getElementById('auth-screen').classList.add('active');
    document.getElementById('portal-screen').classList.remove('active');
    document.getElementById('phone').value = '';
    document.getElementById('code').value = '';
    document.getElementById('code-section').style.display = 'none';
    const submitBtn = document.getElementById('submit-btn');
    submitBtn.innerHTML = 'Получить код';
    submitBtn.disabled = false;
}

async function webViewAuth(actionToken) {
    console.log('WebView auth with token:', actionToken.substring(0, 8) + '...');
    try {
        const response = await fetch('./api/auth/webview', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action_token: actionToken })
        });
        const data = await response.json();
        if (!response.ok) throw new Error(data.error);

        token = data.access_token;
        localStorage.setItem('token', token);
        setWebViewAuth(true);
        return true;
    } catch (err) {
        console.error('WebView auth failed:', err);
        return false;
    }
}

async function checkWebViewToken() {
    const urlParams = new URLSearchParams(window.location.search);
    const actionToken = urlParams.get('token');

    if (actionToken && actionToken.length > 0) {
        console.log('Found token in URL, trying WebView auth');
        window.history.replaceState({}, document.title, window.location.pathname);
        return await webViewAuth(actionToken);
    }
    return false;
}

function getToken() {
    return token;
}

function setToken(newToken) {
    token = newToken;
    localStorage.setItem('token', token);
    setWebViewAuth(false);
}

function setWebViewAuth(flag) {
    isWebViewAuth = flag;
    localStorage.setItem('isWebViewAuth', JSON.stringify(flag));
}

function getWebViewAuth() {
    if (localStorage.getItem('isWebViewAuth') !== null) {
        return JSON.parse(localStorage.getItem('isWebViewAuth'));
    }
    return false;
}

export { getToken, setToken, logout, showMessage, checkWebViewToken, setWebViewAuth, getWebViewAuth };