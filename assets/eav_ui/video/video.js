(() => {
  const videoFields = document.querySelectorAll('[data-eav-video-field]');
  if (videoFields.length === 0) {
    return;
  }

  if (!window.FileManagerModal) {
    console.warn('[eav-video] FileManagerModal is required');
    return;
  }

  const togglePlaceholder = (previewId, placeholderId, showPlaceholder) => {
    const preview = document.getElementById(previewId);
    const placeholder = document.getElementById(placeholderId);
    if (!preview || !placeholder) {
      return;
    }

    if (showPlaceholder) {
      placeholder.classList.remove('d-none');
      preview.classList.add('d-none');
      preview.innerHTML = '';
      return;
    }

    placeholder.classList.add('d-none');
    preview.classList.remove('d-none');
  };

  const renderPreview = (previewId, placeholderId, url) => {
    const preview = document.getElementById(previewId);
    if (!preview) {
      return;
    }

    preview.innerHTML = '';
    const trimmed = (url || '').trim();
    if (trimmed === '') {
      togglePlaceholder(previewId, placeholderId, true);
      return;
    }

    const video = document.createElement('video');
    video.src = trimmed;
    video.className = 'eav-ui-video-player';
    video.controls = true;
    video.loop = true;
    video.muted = true;
    video.playsInline = true;
    video.dataset.placeholderId = placeholderId;

    video.addEventListener('error', () => {
      togglePlaceholder(previewId, placeholderId, true);
    });

    video.addEventListener('loadeddata', () => {
      togglePlaceholder(previewId, placeholderId, false);
    });

    preview.appendChild(video);
  };

  const applySelection = (ctx, url) => {
    const trimmed = (url || '').trim();
    const input = document.getElementById(ctx.inputId);
    if (input) {
      input.value = trimmed;
    }
    renderPreview(ctx.previewId, ctx.placeholderId, trimmed);
  };

  const bindButtons = () => {
    const openButtons = document.querySelectorAll('[data-eav-video-open="true"]');
    openButtons.forEach((btn) => {
      btn.addEventListener('click', (event) => {
        event.preventDefault();
        const ctx = {
          inputId: btn.dataset.eavVideoInput || '',
          previewId: btn.dataset.eavVideoPreview || '',
          placeholderId: btn.dataset.eavVideoPlaceholder || '',
        };

        window.FileManagerModal.open({
          type: 'video',
          filetag: 'eav',
          trigger: 'eav-video',
          onSelect: (url) => applySelection(ctx, url),
        });
      });
    });

    const clearButtons = document.querySelectorAll('.eav-video-clear-btn');
    clearButtons.forEach((btn) => {
      btn.addEventListener('click', (event) => {
        event.preventDefault();
        const inputId = btn.dataset.eavVideoInput;
        const previewId = btn.dataset.eavVideoPreview;
        const placeholderId = btn.dataset.eavVideoPlaceholder;
        const input = document.getElementById(inputId || '');
        if (input) {
          input.value = '';
        }
        togglePlaceholder(previewId, placeholderId, true);
      });
    });
  };

  bindButtons();
})();
