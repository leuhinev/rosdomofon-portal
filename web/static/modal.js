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
        cleanup();
        onConfirm();
    };

    const handleCancel = () => {
        confirmModal.style.display = 'none';
        cleanup();
    };

    const cleanup = () => {
        confirmYes.removeEventListener('click', handleConfirm);
        confirmNo.removeEventListener('click', handleCancel);
        closeConfirm.removeEventListener('click', handleCancel);
        window.removeEventListener('click', outsideClick);
    };

    const outsideClick = (e) => {
        if (e.target === confirmModal) {
            handleCancel();
        }
    };

    confirmYes.addEventListener('click', handleConfirm);
    confirmNo.addEventListener('click', handleCancel);
    closeConfirm.addEventListener('click', handleCancel);
    window.addEventListener('click', outsideClick);
}

function closeModal() {
    document.getElementById('modal').style.display = 'none';
    document.getElementById('modal').classList.remove('photo-gallery-modal');
}

export { showConfirm, closeModal };