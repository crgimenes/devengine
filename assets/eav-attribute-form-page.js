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

    // Handle default_value field based on primitive_kind
    const defaultValueInput = document.getElementById('attr_default_value');

    if (primitiveKindSelect && defaultValueInput) {
        // Set initial state
        updateDefaultValueField();

        primitiveKindSelect.addEventListener('change', updateDefaultValueField);

        function updateDefaultValueField() {
            const kind = primitiveKindSelect.value;
            const hasExistingValue = defaultValueInput.value !== '';

            if (!kind) {
                defaultValueInput.type = 'text';
                if (!hasExistingValue) defaultValueInput.value = '';
                defaultValueInput.disabled = true;
                defaultValueInput.placeholder = 'Selecione um tipo primeiro';
                return;
            }

            defaultValueInput.disabled = false;
            defaultValueInput.required = false;

            switch (kind) {
                case 'BOOL':
                    defaultValueInput.type = 'number';
                    defaultValueInput.step = '1';
                    defaultValueInput.min = '0';
                    defaultValueInput.max = '1';
                    if (!hasExistingValue) defaultValueInput.value = '0';
                    defaultValueInput.placeholder = '0 (false) ou 1 (true)';
                    break;
                case 'INT':
                    defaultValueInput.type = 'text';
                    defaultValueInput.inputMode = 'numeric';
                    defaultValueInput.pattern = '-?[0-9]*';
                    defaultValueInput.removeAttribute('step');
                    defaultValueInput.removeAttribute('min');
                    defaultValueInput.removeAttribute('max');
                    if (!hasExistingValue) defaultValueInput.value = '0';
                    defaultValueInput.placeholder = '0';
                    break;
                case 'REAL':
                    defaultValueInput.type = 'text';
                    defaultValueInput.inputMode = 'decimal';
                    defaultValueInput.pattern = '-?[0-9]*[.,]?[0-9]*';
                    defaultValueInput.removeAttribute('step');
                    defaultValueInput.removeAttribute('min');
                    defaultValueInput.removeAttribute('max');
                    if (!hasExistingValue) defaultValueInput.value = '0.0';
                    defaultValueInput.placeholder = '0.0';
                    break;
                case 'TEXT':
                    defaultValueInput.type = 'text';
                    defaultValueInput.removeAttribute('step');
                    defaultValueInput.removeAttribute('min');
                    defaultValueInput.removeAttribute('max');
                    defaultValueInput.placeholder = 'String vazia';
                    break;
                case 'DATETIME':
                    defaultValueInput.type = 'datetime-local';
                    defaultValueInput.removeAttribute('step');
                    defaultValueInput.removeAttribute('min');
                    defaultValueInput.removeAttribute('max');
                    defaultValueInput.placeholder = 'NULL (deixe vazio para NULL)';
                    defaultValueInput.required = false;
                    break;
                default:
                    defaultValueInput.type = 'text';
                    if (!hasExistingValue) defaultValueInput.value = '';
                    defaultValueInput.disabled = true;
                    defaultValueInput.placeholder = 'Selecione um tipo primeiro';
            }
        }
    }

    // Form validation
    const form = document.querySelector('.needs-validation');
    if (form) {
        form.addEventListener('submit', event => {
            console.log('Form submission started');

            // Ensure default value input is enabled so it is included in the POST request,
            // but only if a type is selected.
            if (defaultValueInput && primitiveKindSelect && primitiveKindSelect.value) {
                defaultValueInput.disabled = false;
            }

            if (!form.checkValidity()) {
                console.log('Form validation failed');
                event.preventDefault();
                event.stopPropagation();
            } else {
                console.log('Form validation passed');
            }
            form.classList.add('was-validated');
        }, false);
    }
});
