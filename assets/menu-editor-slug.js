// Auto-generate machine_name from label field (slugify)
// Used in menu editor for both new menus and new items
document.addEventListener('DOMContentLoaded', function () {
    // For new menu form
    setupSlugify('label', 'machine_name');

    // For new item form (inline)
    setupSlugify('new_label', 'new_machine_name');
});

function setupSlugify(labelId, machineNameId) {
    const labelInput = document.getElementById(labelId);
    const machineNameInput = document.getElementById(machineNameId);

    if (!labelInput || !machineNameInput) {
        return; // Elements not found, exit gracefully
    }

    // Auto-slugify on blur, only if machine_name is empty
    labelInput.addEventListener('blur', function () {
        if (machineNameInput.value.trim() === '') {
            machineNameInput.value = slugify(this.value);
        }
    });

    // Also slugify on input for immediate feedback (optional)
    labelInput.addEventListener('input', function () {
        if (machineNameInput.value.trim() === '' || machineNameInput.dataset.autoFilled === 'true') {
            machineNameInput.value = slugify(this.value);
            machineNameInput.dataset.autoFilled = 'true';
        }
    });

    // Mark as manually edited when user types in machine_name
    machineNameInput.addEventListener('input', function () {
        if (this.value !== slugify(labelInput.value)) {
            this.dataset.autoFilled = 'false';
        }
    });
}

function slugify(text) {
    return text
        .toString()
        .toLowerCase()
        .trim()
        .normalize('NFD')                   // Decompose characters
        .replace(/[\u0300-\u036f]/g, '')    // Remove diacritics
        .replace(/[^a-z0-9\s_]/g, '')       // Remove invalid chars
        .replace(/\s+/g, '_')                // Replace spaces with _
        .replace(/_+/g, '_')                 // Replace multiple _ with single _
        .replace(/^_|_$/g, '');              // Remove leading/trailing _
}
