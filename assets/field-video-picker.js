/**
 * field-video-picker.js
 * Handles video field interactions with the filemanager modal
 */

document.addEventListener('DOMContentLoaded', function () {
    // Track which field is currently being edited
    let activeVideoFieldName = null;

    // Handle select button clicks for video fields
    // Handle select button or placeholder clicks for video fields
    document.addEventListener('click', function (e) {
        const selectBtn = e.target.closest('.field-video-select-btn, .field-video-placeholder');
        if (selectBtn && !selectBtn.classList.contains('readonly')) {
            activeVideoFieldName = selectBtn.dataset.fieldName;

            // Configure modal for videos
            const modal = document.getElementById('fileManagerModal');
            if (modal) {
                const titleEl = modal.querySelector('[data-filemanager-title]');
                const acceptHelp = modal.querySelector('[data-filemanager-accept-help]');
                const fileInput = modal.querySelector('#fileManagerFileInput');

                if (titleEl) titleEl.textContent = 'Selecionar Vídeo';
                if (acceptHelp) acceptHelp.textContent = 'Formatos: MP4, WebM, OGG';
                if (fileInput) fileInput.setAttribute('accept', 'video/*');

                // Load user videos
                loadUserVideos();
            }
        }
    });

    // Handle remove button clicks for video fields
    document.addEventListener('click', function (e) {
        const removeBtn = e.target.closest('.field-video-remove-btn');
        if (removeBtn) {
            const fieldName = removeBtn.dataset.fieldName;
            setVideoValue(fieldName, '');
        }
    });

    // Handle file card clicks in the modal for videos
    document.addEventListener('click', function (e) {
        if (!activeVideoFieldName) return;

        const fileCard = e.target.closest('[data-file-url]');
        if (fileCard) {
            const url = fileCard.dataset.fileUrl;
            if (url) {
                setVideoValue(activeVideoFieldName, url);
                closeModal();
            }
        }
    });

    // Handle URL insert button for videos
    const insertUrlBtn = document.getElementById('fileManagerInsertUrlBtn');
    if (insertUrlBtn) {
        const originalHandler = insertUrlBtn.onclick;
        insertUrlBtn.addEventListener('click', function () {
            if (!activeVideoFieldName) return;

            const urlInput = document.getElementById('fileManagerUrlInput');
            if (urlInput && urlInput.value) {
                setVideoValue(activeVideoFieldName, urlInput.value);
                urlInput.value = '';
                closeModal();
            }
        });
    }
    // Handle upload form submission for videos
    const uploadForm = document.getElementById('fileManagerUploadForm');
    if (uploadForm) {
        uploadForm.addEventListener('submit', function (e) {
            if (!activeVideoFieldName) return; // Only handle if video field is active

            e.preventDefault();
            e.stopImmediatePropagation(); // Prevent image picker from also handling this
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
                        if (statusEl) statusEl.innerHTML = '<div class="text-success">Upload concluído!</div>';
                        if (fileInput) fileInput.value = '';
                        if (description) description.value = '';
                        // Reload videos after a short delay
                        setTimeout(loadUserVideos, 500);
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
    // Pagination state for videos
    let videoOffset = 0;
    const videoLimit = 20;
    let isLoadingVideos = false;
    let hasMoreVideos = true;

    /**
     * Load user videos into the modal list with infinite scroll
     */
    function loadUserVideos(append = false) {
        const listEl = document.getElementById('fileManagerList');
        if (!listEl || isLoadingVideos) return;

        if (!append) {
            videoOffset = 0;
            hasMoreVideos = true;
            listEl.innerHTML = '<div class="text-center text-body-secondary py-4"><small>Carregando...</small></div>';
        }

        if (!hasMoreVideos) return;

        isLoadingVideos = true;

        fetch(`/api/files?offset=${videoOffset}&limit=${videoLimit}`)
            .then(r => r.json())
            .then(data => {
                const files = data.data || [];

                // Filter for videos only
                const videos = files.filter(function (f) {
                    const type = (f.filetype || '').toLowerCase();
                    return type.startsWith('video/');
                });

                // Check if there are more files to load
                hasMoreVideos = files.length === videoLimit;
                videoOffset += files.length;

                if (videos.length === 0 && !append) {
                    listEl.innerHTML = '<div class="text-center text-body-secondary py-4"><small>Nenhum vídeo encontrado</small></div>';
                    return;
                }

                let html = '';
                videos.forEach(function (file) {
                    const url = file.file_url || '';
                    const name = file.display_name || file.filename || 'Vídeo';
                    const desc = file.description || '';
                    const size = formatFileSize(file.filesize || 0);
                    const date = file.created_at ? file.created_at.split('T')[0] : '';

                    html += `
                        <div class="card mb-2 card-hover" data-file-url="${url}" role="button" style="cursor: pointer;">
                            <div class="row g-0">
                                <div class="col-4">
                                    <video src="${url}" class="rounded-start h-100 w-100" 
                                           style="object-fit: contain; background: var(--bs-body-bg); min-height: 60px; max-height: 80px;"
                                           preload="metadata" autoplay playsinline webkit-playsinline muted loop></video>
                                </div>
                                <div class="col-8">
                                    <div class="card-body p-2">
                                        <h6 class="card-title mb-1 text-truncate small">${name}</h6>
                                        ${desc ? `<p class="card-text small text-muted mb-1 text-truncate">${desc}</p>` : ''}
                                        <p class="card-text"><small class="text-body-secondary">${size} • ${date}</small></p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    `;
                });

                if (append) {
                    const loadingIndicator = listEl.querySelector('.loading-more');
                    if (loadingIndicator) loadingIndicator.remove();
                    listEl.insertAdjacentHTML('beforeend', html);
                } else {
                    listEl.innerHTML = html;
                }

                if (hasMoreVideos) {
                    listEl.insertAdjacentHTML('beforeend', '<div class="loading-more text-center py-2"><small class="text-muted">Role para carregar mais...</small></div>');
                }
            })
            .catch(err => {
                console.error('Error loading videos:', err);
                if (!append) {
                    listEl.innerHTML = '<div class="text-center text-danger py-4"><small>Erro ao carregar vídeos</small></div>';
                }
            })
            .finally(() => {
                isLoadingVideos = false;
            });
    }

    // Infinite scroll for video list
    document.getElementById('fileManagerList')?.addEventListener('scroll', function () {
        if (!activeVideoFieldName) return;
        const el = this;
        if (el.scrollTop + el.clientHeight >= el.scrollHeight - 50) {
            if (hasMoreVideos && !isLoadingVideos) {
                loadUserVideos(true);
            }
        }
    });

    /**
     * Format file size in human readable format
     */
    function formatFileSize(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
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
        activeVideoFieldName = null;
    }

    /**
     * Set video value and update preview
     */
    function setVideoValue(fieldName, url) {
        const input = document.getElementById(fieldName);
        const previewWrapper = document.getElementById(fieldName + '_preview_wrapper');
        const removeBtn = document.querySelector(`.field-video-remove-btn[data-field-name="${fieldName}"]`);

        if (input) {
            input.value = url;
        }

        if (previewWrapper) {
            if (url) {
                // Get options from container
                const container = previewWrapper.closest('.field-video-container');
                let options = { controls: true, autoplay: false, loop: false, muted: false };

                if (container && container.dataset.videoOptions) {
                    try {
                        // The Go template outputs map[key:value], which is not valid JSON
                        // We need to parse it manually or rely on proper JSON format from server
                        // Since we can't easily parse Go map format in JS, we'll try to guess or use defaults
                        // BETTER APPROACH: The server should output valid JSON or data attributes
                        // For now, let's assume standard behavior: show controls in preview so user can test
                        // BUT user wants options applied.

                        // Let's parse the map string manually loosely: map[key:value ...]
                        const metaStr = container.dataset.videoOptions;
                        if (metaStr.startsWith('map[')) {
                            options.controls = metaStr.includes('controls:true');
                            options.autoplay = metaStr.includes('autoplay:true');
                            options.loop = metaStr.includes('loop:true');
                            options.muted = metaStr.includes('muted:true');
                        }
                    } catch (e) {
                        console.error('Error parsing video options:', e);
                    }
                }

                const attrs = [
                    options.controls ? 'controls' : '',
                    options.autoplay ? 'autoplay' : '',
                    options.loop ? 'loop' : '',
                    options.muted ? 'muted' : '',
                    'playsinline',
                    'class="field-video-preview rounded w-100"',
                    `id="${fieldName}_preview"`
                ].filter(Boolean).join(' ');

                previewWrapper.innerHTML = `<video src="${url}" ${attrs}></video>`;
            } else {
                previewWrapper.innerHTML = `
                    <div class="field-video-placeholder text-center text-muted p-4 border rounded bg-body-secondary">
                        <svg class="mb-2" width="48" height="48" fill="currentColor" viewBox="0 0 16 16">
                            <path d="M0 1a1 1 0 0 1 1-1h14a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1H1a1 1 0 0 1-1-1V1zm4 0v6h8V1H4zm8 8H4v6h8V9zM1 1v2h2V1H1zm2 3H1v2h2V4zM1 7v2h2V7H1zm2 3H1v2h2v-2zm-2 3v2h2v-2H1zM15 1h-2v2h2V1zm-2 3v2h2V4h-2zm2 3h-2v2h2V7zm-2 3v2h2v-2h-2zm2 3h-2v2h2v-2z"/>
                        </svg>
                        <div class="small">Nenhum vídeo selecionado</div>
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
