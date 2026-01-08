/**
 * field-image-picker.js
 * Handles image field interactions with the filemanager modal
 */

document.addEventListener('DOMContentLoaded', function () {
    // Track which field is currently being edited
    let activeFieldName = null;

    // Handle select button clicks
    document.addEventListener('click', function (e) {
        const selectBtn = e.target.closest('.field-image-select-btn');
        if (selectBtn) {
            activeFieldName = selectBtn.dataset.fieldName;

            // Configure modal for images
            const modal = document.getElementById('fileManagerModal');
            if (modal) {
                const titleEl = modal.querySelector('[data-filemanager-title]');
                const acceptHelp = modal.querySelector('[data-filemanager-accept-help]');
                const fileInput = modal.querySelector('#fileManagerFileInput');

                if (titleEl) titleEl.textContent = 'Selecionar Imagem';
                if (acceptHelp) acceptHelp.textContent = 'Formatos: JPG, PNG, GIF, WebP, SVG';
                if (fileInput) fileInput.setAttribute('accept', 'image/*');

                // Load user images
                loadUserImages();
            }
        }
    });

    // Handle remove button clicks
    document.addEventListener('click', function (e) {
        const removeBtn = e.target.closest('.field-image-remove-btn');
        if (removeBtn) {
            const fieldName = removeBtn.dataset.fieldName;
            setImageValue(fieldName, '', '');
        }
    });

    // Handle file card clicks in the modal
    document.addEventListener('click', function (e) {
        const fileCard = e.target.closest('[data-file-url]');
        if (fileCard && activeFieldName) {
            const url = fileCard.dataset.fileUrl;
            const img = fileCard.querySelector('img');
            const alt = img ? img.alt : '';
            if (url) {
                setImageValue(activeFieldName, url, alt);
                closeModal();
            }
        }
    });

    // Handle URL insert button
    const insertUrlBtn = document.getElementById('fileManagerInsertUrlBtn');
    if (insertUrlBtn) {
        insertUrlBtn.addEventListener('click', function () {
            const urlInput = document.getElementById('fileManagerUrlInput');
            if (urlInput && activeFieldName && urlInput.value) {
                setImageValue(activeFieldName, urlInput.value, '');
                urlInput.value = '';
                closeModal();
            }
        });
    }

    // Handle upload form submission
    const uploadForm = document.getElementById('fileManagerUploadForm');
    if (uploadForm) {
        uploadForm.addEventListener('submit', function (e) {
            e.preventDefault();
            const fileInput = document.getElementById('fileManagerFileInput');
            const description = document.getElementById('fileManagerDescription');
            const statusEl = document.getElementById('fileManagerUploadStatus');
            const uploadBtn = document.getElementById('fileManagerUploadBtn');

            if (!fileInput || !fileInput.files.length) {
                if (statusEl) statusEl.innerHTML = '<div class="text-warning">Selecione um arquivo</div>';
                return;
            }

            const formData = new FormData();
            formData.append('file', fileInput.files[0]);
            if (description && description.value) {
                formData.append('description', description.value);
            }

            if (statusEl) statusEl.innerHTML = '<div class="text-info"><span class="spinner-border spinner-border-sm me-2"></span>Enviando...</div>';
            if (uploadBtn) uploadBtn.disabled = true;

            fetch('/files/upload', {
                method: 'POST',
                body: formData,
                credentials: 'same-origin'
            })
                .then(response => {
                    if (response.ok || response.redirected) {
                        // Upload successful - reload images
                        if (statusEl) statusEl.innerHTML = '<div class="text-success">Upload concluído!</div>';
                        if (fileInput) fileInput.value = '';
                        if (description) description.value = '';
                        // Reload images after a short delay
                        setTimeout(loadUserImages, 500);
                    } else {
                        throw new Error('Upload falhou');
                    }
                })
                .catch(err => {
                    console.error('Upload error:', err);
                    if (statusEl) statusEl.innerHTML = '<div class="text-danger">Erro ao enviar arquivo</div>';
                })
                .finally(() => {
                    if (uploadBtn) uploadBtn.disabled = false;
                });
        });
    }

    /**
     * Load user images into the modal list
     */
    function loadUserImages() {
        const listEl = document.getElementById('fileManagerList');
        if (!listEl) return;

        listEl.innerHTML = '<div class="text-center text-body-secondary py-4"><small>Carregando...</small></div>';

        fetch('/api/files?limit=100')
            .then(r => r.json())
            .then(data => {
                // API returns { data: [...], offset, limit, total }
                const files = data.data || [];

                // Filter for images only
                const images = files.filter(function (f) {
                    const type = (f.filetype || '').toLowerCase();
                    return type.startsWith('image/');
                });

                if (images.length === 0) {
                    listEl.innerHTML = '<div class="text-center text-body-secondary py-4"><small>Nenhuma imagem encontrada</small></div>';
                    return;
                }

                let html = '<div class="row g-2">';
                images.forEach(function (file) {
                    const url = file.file_url || '';
                    const alt = file.description || file.display_name || 'Imagem';
                    html += `
                        <div class="col-4 col-md-3">
                            <div class="card card-hover h-100" data-file-url="${url}" role="button">
                                <img src="${url}" class="card-img-top" alt="${alt}" 
                                     style="width: 100%; height: 80px; object-fit: contain; background: var(--bs-body-bg);">
                            </div>
                        </div>
                    `;
                });
                html += '</div>';
                listEl.innerHTML = html;
            })
            .catch(err => {
                console.error('Error loading images:', err);
                listEl.innerHTML = '<div class="text-center text-danger py-4"><small>Erro ao carregar imagens</small></div>';
            });
    }

    /**
     * Close the modal
     */
    function closeModal() {
        const modalEl = document.getElementById('fileManagerModal');
        if (modalEl) {
            const modal = bootstrap.Modal.getInstance(modalEl);
            if (modal) modal.hide();
        }
        activeFieldName = null;
    }

    /**
     * Set image value and update preview
     */
    function setImageValue(fieldName, url, alt) {
        const input = document.getElementById(fieldName);
        const previewWrapper = document.getElementById(fieldName + '_preview_wrapper');
        const removeBtn = document.querySelector(`.field-image-remove-btn[data-field-name="${fieldName}"]`);
        const altText = alt || 'Imagem';

        if (input) {
            input.value = url;
        }

        if (previewWrapper) {
            if (url) {
                previewWrapper.innerHTML = `<img src="${url}" alt="${altText}" class="field-image-preview rounded" id="${fieldName}_preview">`;
            } else {
                previewWrapper.innerHTML = `
                    <div class="field-image-placeholder text-center text-muted p-4 border rounded bg-body-secondary">
                        <svg class="mb-2" width="48" height="48" fill="currentColor" viewBox="0 0 16 16">
                            <path d="M6.002 5.5a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0z"/>
                            <path d="M2.002 1a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V3a2 2 0 0 0-2-2h-12zm12 1a1 1 0 0 1 1 1v6.5l-3.777-1.947a.5.5 0 0 0-.577.093l-3.71 3.71-2.66-1.772a.5.5 0 0 0-.63.062L1.002 12V3a1 1 0 0 1 1-1h12z"/>
                        </svg>
                        <div class="small">Nenhuma imagem selecionada</div>
                    </div>
                `;
            }
        }

        if (removeBtn) {
            if (url) {
                removeBtn.classList.remove('d-none');
            } else {
                removeBtn.classList.add('d-none');
            }
        }
    }
});
