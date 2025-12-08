(() => {
  const audioFields = document.querySelectorAll('[data-eav-audio-field]');
  if (audioFields.length === 0) {
    return;
  }

  if (!window.FileManagerModal) {
    console.warn('[eav-audio] FileManagerModal is required');
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

    const audio = document.createElement('audio');
    audio.src = trimmed;
    audio.className = 'w-100 eav-ui-audio-player';
    audio.controls = true;
    audio.dataset.placeholderId = placeholderId;

    audio.addEventListener('error', () => {
      togglePlaceholder(previewId, placeholderId, true);
    });

    audio.addEventListener('loadeddata', () => {
      togglePlaceholder(previewId, placeholderId, false);
    });

    preview.appendChild(audio);
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
    const openButtons = document.querySelectorAll('[data-eav-audio-open="true"]');
    openButtons.forEach((btn) => {
      btn.addEventListener('click', (event) => {
        event.preventDefault();
        const ctx = {
          inputId: btn.dataset.eavAudioInput || '',
          previewId: btn.dataset.eavAudioPreview || '',
          placeholderId: btn.dataset.eavAudioPlaceholder || '',
        };

        window.FileManagerModal.open({
          type: 'audio',
          filetag: 'eav',
          trigger: 'eav-audio',
          onSelect: (url) => applySelection(ctx, url),
        });
      });
    });

    const clearButtons = document.querySelectorAll('.eav-audio-clear-btn');
    clearButtons.forEach((btn) => {
      btn.addEventListener('click', (event) => {
        event.preventDefault();
        const inputId = btn.dataset.eavAudioInput;
        const previewId = btn.dataset.eavAudioPreview;
        const placeholderId = btn.dataset.eavAudioPlaceholder;
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
