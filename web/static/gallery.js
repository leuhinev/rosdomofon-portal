import { closeModal } from './modal.js';

function showPhotoGallery(photoData) {
    if (!photoData) {
        alert('Нет фотографии для этого автомобиля');
        return;
    }

    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
        <div class="photo-gallery">
            <img src="${photoData}" class="gallery-img" alt="Фото автомобиля">
        </div>
    `;
    document.getElementById('modal').style.display = 'block';
    document.getElementById('modal').classList.add('photo-gallery-modal');
}

export { showPhotoGallery };