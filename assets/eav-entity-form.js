// Auto-generate machine_name from name field (slugify)
document.addEventListener('DOMContentLoaded', function () {
    const nameInput = document.getElementById('name');
    const machineNameInput = document.getElementById('machine_name');

    if (!nameInput || !machineNameInput) {
        return; // Elements not found, exit gracefully
    }

    // Auto-slugify on blur, only if machine_name is empty
    nameInput.addEventListener('blur', function () {
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

    // Bootstrap form validation
    const forms = document.querySelectorAll('.needs-validation');
    Array.from(forms).forEach(form => {
        form.addEventListener('submit', event => {
            if (!form.checkValidity()) {
                event.preventDefault();
                event.stopPropagation();
            }
            form.classList.add('was-validated');
        }, false);
    });
});
