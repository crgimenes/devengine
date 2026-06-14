/**
 * field-audio-picker.js
 * Handles audio field interactions with the filemanager modal
 */

document.addEventListener('DOMContentLoaded', function () {
    // Track which field is currently being edited
    let activeAudioFieldName = null;

    // Handle select button or placeholder clicks for audio fields
    document.addEventListener('click', function (e) {
        const selectBtn = e.target.closest('.field-audio-select-btn, .field-audio-placeholder');
        if (selectBtn && !selectBtn.classList.contains('readonly')) {
            activeAudioFieldName = selectBtn.dataset.fieldName;

            // Configure modal for audio
            const modal = document.getElementById('fileManagerModal');
            if (modal) {
                const titleEl = modal.querySelector('[data-filemanager-title]');
                const acceptHelp = modal.querySelector('[data-filemanager-accept-help]');
                const fileInput = modal.querySelector('#fileManagerFileInput');

                if (titleEl) titleEl.textContent = 'Select Audio';
                if (acceptHelp) acceptHelp.textContent = 'Formats: MP3, WAV, OGG, M4A';
                if (fileInput) fileInput.setAttribute('accept', 'audio/*');

                // Load user audios
                loadUserAudios();
            }
        }
    });

    // Handle remove button clicks for audio fields
    document.addEventListener('click', function (e) {
        const removeBtn = e.target.closest('.field-audio-remove-btn');
        if (removeBtn) {
            const fieldName = removeBtn.dataset.fieldName;
            setAudioValue(fieldName, '');
        }
    });

    // Handle file card clicks in the modal for audio
    document.addEventListener('click', function (e) {
        if (!activeAudioFieldName) return;

        // Ignore clicks on audio controls themselves to allow playback
        if (e.target.tagName.toLowerCase() === 'audio' || e.target.closest('audio')) {
            return;
        }

        const fileCard = e.target.closest('[data-file-url]');
        if (fileCard) {
            // Check if we are in audio mode by checking the title
            const modal = document.getElementById('fileManagerModal');
            const titleEl = modal ? modal.querySelector('[data-filemanager-title]') : null;
            if (titleEl && titleEl.textContent === 'Select Audio') {
                const url = fileCard.dataset.fileUrl;
                if (url) {
                    setAudioValue(activeAudioFieldName, url);
                    closeModal();
                }
            }
        }
    });

    // Exclusive Playback Logic & Stop on Close
    const modalEl = document.getElementById('fileManagerModal');
    if (modalEl) {
        // Handle upload form submission
        const uploadForm = document.getElementById('fileManagerUploadForm');
        if (uploadForm) {
            uploadForm.addEventListener('submit', function (e) {
                if (!activeAudioFieldName) return; // Only process if we are picking audio

                e.preventDefault();
                e.stopImmediatePropagation(); // Prevent other pickers from handling this

                const fileInput = document.getElementById('fileManagerFileInput');
                const description = document.getElementById('fileManagerDescription');
                const statusEl = document.getElementById('fileManagerUploadStatus');
                const uploadBtn = document.getElementById('fileManagerUploadBtn');

                if (!fileInput || !fileInput.files.length) {
                    if (statusEl) statusEl.innerHTML = '<div class="text-warning">Select a file</div>';
                    return;
                }

                const formData = new FormData();
                formData.append('file', fileInput.files[0]);
                if (description && description.value) {
                    formData.append('description', description.value);
                }

                if (statusEl) statusEl.innerHTML = '<div class="text-info"><span class="spinner-border spinner-border-sm me-2"></span>Uploading...</div>';
                if (uploadBtn) uploadBtn.disabled = true;

                fetch('/files/upload', {
                    method: 'POST',
                    body: formData,
                    credentials: 'same-origin'
                })
                    .then(response => {
                        if (response.ok || response.redirected) {
                            // Upload successful
                            if (statusEl) statusEl.innerHTML = '<div class="text-success">Upload complete!</div>';
                            if (fileInput) fileInput.value = '';
                            if (description) description.value = '';

                            // Reload audios after a short delay
                            setTimeout(loadUserAudios, 500);
                        } else {
                            throw new Error('Upload failed');
                        }
                    })
                    .catch(err => {
                        console.error('Upload error:', err);
                        if (statusEl) statusEl.innerHTML = '<div class="text-danger">Could not upload file</div>';
                    })
                    .finally(() => {
                        if (uploadBtn) uploadBtn.disabled = false;
                    });
            });
        }

        // Stop all audio when modal closes
        modalEl.addEventListener('hidden.bs.modal', function () {
            const audios = modalEl.querySelectorAll('audio');
            audios.forEach(a => {
                a.pause();
                a.currentTime = 0;
            });
            activeAudioFieldName = null;
        });

        // Use event delegation for exclusive playback (capture phase)
        modalEl.addEventListener('play', function (e) {
            if (e.target.tagName.toLowerCase() === 'audio') {
                const allAudios = modalEl.querySelectorAll('audio');
                allAudios.forEach(audio => {
                    if (audio !== e.target) {
                        audio.pause();
                    }
                });
            }
        }, true);
    }

    // Pagination state for audios
    let audioOffset = 0;
    const audioLimit = 20;
    let isLoadingAudios = false;
    let hasMoreAudios = true;

    /**
     * Load user audios into the modal list with infinite scroll
     */
    function loadUserAudios(append = false) {
        const listEl = document.getElementById('fileManagerList');
        // Ensure we are in the correct context/modal usage
        const modal = document.getElementById('fileManagerModal');
        const titleEl = modal ? modal.querySelector('[data-filemanager-title]') : null;
        if (titleEl && titleEl.textContent !== 'Select Audio') return;

        if (!listEl || isLoadingAudios) return;

        if (!append) {
            audioOffset = 0;
            hasMoreAudios = true;
            listEl.innerHTML = '<div class="text-center text-body-secondary py-4"><small>Loading...</small></div>';
        }

        if (!hasMoreAudios) return;

        isLoadingAudios = true;

        fetch(`/api/files?offset=${audioOffset}&limit=${audioLimit}`)
            .then(r => r.json())
            .then(data => {
                const files = data.data || [];

                // Filter for audios only
                // Note: file.filetype is what we want, not mime_type
                const audios = files.filter(function (f) {
                    const type = (f.filetype || '').toLowerCase();
                    return type.startsWith('audio/');
                });

                // Check if there are more files to load (based on raw files returned)
                // If we filter client side, pagination logic can be tricky, but we follow standard pattern
                // Just assuming if we got full page, there might be more.
                hasMoreAudios = files.length === audioLimit;
                audioOffset += files.length;

                if (audios.length === 0 && !append) {
                    // Only show empty message if we strictly have no audios after filter
                    // If we got files but filtered all out, we might want to automatically fetch next page,
                    // but for simplicity mirroring image picker, we just show "No audios found" if first page yields none.
                    listEl.innerHTML = '<div class="text-center text-body-secondary py-4"><small>No audio files found</small></div>';
                    return;
                }

                let html = '';
                audios.forEach(function (file) {
                    const url = file.file_url || '';
                    const name = file.display_name || file.filename || 'Audio';
                    const desc = file.description || '';
                    const size = formatFileSize(file.filesize || 0);
                    const date = file.created_at ? file.created_at.split('T')[0] : '';

                    html += `
                        <div class="card mb-2 card-hover" data-file-url="${url}" role="button" style="cursor: pointer;">
                            <div class="row g-0 align-items-center">
                                <div class="col-auto text-center bg-body-secondary rounded-start p-2" style="min-width: 60px;">
                                    <audio src="${url}" controls preload="none" style="width: 50px; height: 50px;"></audio>
                                </div>
                                <div class="col">
                                    <div class="card-body p-2">
                                        <div class="d-flex justify-content-between align-items-start">
                                            <h6 class="card-title text-truncate mb-1" title="${name}">${name}</h6>
                                            <span class="badge bg-secondary rounded-pill" style="font-size: 0.6rem;">Audio</span>
                                        </div>
                                        <p class="card-text"><small class="text-body-secondary">${size} • ${date}</small></p>
                                        ${desc ? `<p class="card-text small text-muted text-truncate mb-0">${desc}</p>` : ''}
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

                if (hasMoreAudios) {
                    listEl.insertAdjacentHTML('beforeend', '<div class="loading-more text-center py-2"><small class="text-muted">Scroll to load more...</small></div>');
                }
            })
            .catch(err => {
                console.error('Error loading audios:', err);
                if (!append) {
                    listEl.innerHTML = '<div class="text-center text-danger py-4"><small>Could not load audio files</small></div>';
                }
            })
            .finally(() => {
                isLoadingAudios = false;
            });
    }

    // Infinite scroll for audio list
    document.getElementById('fileManagerList')?.addEventListener('scroll', function () {
        if (!activeAudioFieldName) return;
        const el = this;
        if (el.scrollTop + el.clientHeight >= el.scrollHeight - 50) {
            if (hasMoreAudios && !isLoadingAudios) {
                loadUserAudios(true);
            }
        }
    });


    function formatFileSize(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    function closeModal() {
        const modalEl = document.getElementById('fileManagerModal');
        if (modalEl) {
            const modal = bootstrap.Modal.getInstance(modalEl);
            if (modal) {
                modal.hide();
            } else {
                const closeBtn = modalEl.querySelector('.btn-close');
                if (closeBtn) closeBtn.click();
            }
        }
        // Don't null activeAudioFieldName here immediately if we need it for cleanup, 
        // but the hidden.bs.modal listener handles cleanup better. 
        // We leave it to the listener to be consistent.
    }

    /**
     * Set audio value and update preview
     */
    function setAudioValue(fieldName, url) {
        const input = document.getElementById(fieldName);
        const previewWrapper = document.getElementById(fieldName + '_preview_wrapper');
        const removeBtn = document.querySelector(`.field-audio-remove-btn[data-field-name="${fieldName}"]`);

        if (input) {
            input.value = url;
        }

        if (previewWrapper) {
            if (url) {
                // Get options from container
                const container = previewWrapper.closest('.field-audio-container');
                let options = { controls: true, autoplay: false, loop: false, muted: false };

                if (container && container.dataset.audioOptions) {
                    try {
                        const metaStr = container.dataset.audioOptions;
                        if (metaStr.startsWith('map[')) {
                            options.controls = metaStr.includes('controls:true');
                            options.autoplay = metaStr.includes('autoplay:true');
                            options.loop = metaStr.includes('loop:true');
                            options.muted = metaStr.includes('muted:true');
                        }
                    } catch (e) {
                        console.error('Error parsing audio options:', e);
                    }
                }

                const attrs = [
                    options.controls ? 'controls' : '',
                    options.autoplay ? 'autoplay' : '',
                    options.loop ? 'loop' : '',
                    options.muted ? 'muted' : '',
                    'class="field-audio-preview w-100"',
                    `id="${fieldName}_preview"`
                ].filter(Boolean).join(' ');

                previewWrapper.innerHTML = `<audio src="${url}" ${attrs}></audio>`;
            } else {
                previewWrapper.innerHTML = `
                    <div class="field-audio-placeholder text-center text-muted p-4 border rounded bg-body-secondary"
                         role="button" data-bs-toggle="modal" data-bs-target="#fileManagerModal"
                         data-field-name="${fieldName}" style="cursor: pointer;" title="Click to select an audio file">
                        <svg class="mb-2" width="48" height="48" fill="currentColor" viewBox="0 0 16 16">
                            <path d="M6 13c0 1.105-1.12 2-2.5 2S1 14.105 1 13c0-1.104 1.12-2 2.5-2s2.5.896 2.5 2zm9-2c0 1.105-1.12 2-2.5 2s-2.5-.895-2.5-2 1.12-2 2.5-2 2.5.895 2.5 2z"/>
                            <path fill-rule="evenodd" d="M14 11V2h1v9h-1zM6 3v10H5V3h1z"/>
                            <path d="M5 2.905a1 1 0 0 1 .9-.995l8-.8a1 1 0 0 1 1.1.995V3L5 4V2.905z"/>
                        </svg>
                        <div class="small">No audio selected</div>
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
