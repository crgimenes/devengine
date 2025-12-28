// Auto-generate machine_name from label on blur (only if empty and not editing)
document.addEventListener('DOMContentLoaded', () => {
    const labelInput = document.getElementById('attr_label');
    const machineNameInput = document.getElementById('attr_machine_name');

    if (labelInput && machineNameInput && !machineNameInput.readOnly) {
        labelInput.addEventListener('blur', () => {
            if (machineNameInput.value.trim() === '') {
                const slug = labelInput.value
                    .toLowerCase()
                    .normalize('NFD')
                    .replace(/[\u0300-\u036f]/g, '')
                    .replace(/[^a-z0-9]+/g, '_')
                    .replace(/^_+|_+$/g, '');
                machineNameInput.value = slug;
            }
        });
    }

    // Show/hide max_length field based on primitive_kind
    const primitiveKindSelect = document.getElementById('attr_primitive_kind');
    const maxLengthContainer = document.getElementById('max_length_container');

    if (primitiveKindSelect && maxLengthContainer) {
        primitiveKindSelect.addEventListener('change', () => {
            if (primitiveKindSelect.value === 'TEXT') {
                maxLengthContainer.style.display = 'block';
            } else {
                maxLengthContainer.style.display = 'none';
            }
        });
    }

    // Form validation
    const form = document.querySelector('.needs-validation');
    if (form) {
        form.addEventListener('submit', event => {
            if (!form.checkValidity()) {
                event.preventDefault();
                event.stopPropagation();
            }
            form.classList.add('was-validated');
        }, false);
    }
});
