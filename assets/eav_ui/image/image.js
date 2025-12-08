(() => {
  const imageFields = document.querySelectorAll('[data-eav-image-field]');
  if (imageFields.length === 0) {
    return;
  }

  if (!window.FileManagerModal) {
    console.warn('[eav-image] FileManagerModal is required');
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

    const img = document.createElement('img');
    img.src = trimmed;
    img.alt = 'Imagem selecionada';
    img.className = 'img-fluid rounded eav-ui-image-preview';
    img.loading = 'lazy';
    img.dataset.placeholderId = placeholderId;

    img.addEventListener('error', () => {
      togglePlaceholder(previewId, placeholderId, true);
    });

    img.addEventListener('load', () => {
      togglePlaceholder(previewId, placeholderId, false);
    });

    preview.appendChild(img);
  };

  const attachPreviewErrorHandlers = (root) => {
    const previews = root.querySelectorAll('.eav-ui-image-preview');
    previews.forEach((img) => {
      const placeholderId = img.dataset.placeholderId;
      const previewId = img.parentElement ? img.parentElement.id : '';
      if (!placeholderId || !previewId) {
        return;
      }

      const fallback = () => {
        togglePlaceholder(previewId, placeholderId, true);
      };

      img.addEventListener('error', fallback);
      if (img.complete && img.naturalWidth === 0) {
        fallback();
      }
    });
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
    const openButtons = document.querySelectorAll('[data-eav-image-open="true"]');
    openButtons.forEach((btn) => {
      btn.addEventListener('click', (event) => {
        event.preventDefault();
        const ctx = {
          inputId: btn.dataset.eavImageInput || '',
          previewId: btn.dataset.eavImagePreview || '',
          placeholderId: btn.dataset.eavImagePlaceholder || '',
        };

        window.FileManagerModal.open({
          type: 'image',
          filetag: 'eav',
          trigger: 'eav-image',
          onSelect: (url) => applySelection(ctx, url),
        });
      });
    });

    const clearButtons = document.querySelectorAll('.eav-image-clear-btn');
    clearButtons.forEach((btn) => {
      btn.addEventListener('click', (event) => {
        event.preventDefault();
        const inputId = btn.dataset.eavImageInput;
        const previewId = btn.dataset.eavImagePreview;
        const placeholderId = btn.dataset.eavImagePlaceholder;
        const input = document.getElementById(inputId || '');
        if (input) {
          input.value = '';
        }
        togglePlaceholder(previewId, placeholderId, true);
      });
    });
  };

  bindButtons();
  attachPreviewErrorHandlers(document);
})();
