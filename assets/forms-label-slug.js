// Auto-generate machine_name from label field (slugify)
// Reusable for forms that have label and machine_name fields
document.addEventListener('DOMContentLoaded', function () {
    const labelInput = document.getElementById('label');
    const machineNameInput = document.getElementById('machine_name');

    if (!labelInput || !machineNameInput) {
        return; // Elements not found, exit gracefully
    }

    // Auto-slugify on blur, only if machine_name is empty
    labelInput.addEventListener('blur', function () {
        if (machineNameInput.value.trim() === '') {
            machineNameInput.value = slugify(this.value);
        }
    });

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
});
