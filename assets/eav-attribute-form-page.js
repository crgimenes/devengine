// Auto-generate machine_name from label (skipped on read-only edit forms).
document.addEventListener('DOMContentLoaded', () => {
    const labelInput = document.getElementById('attr_label');
    const machineNameInput = document.getElementById('attr_machine_name');

    if (machineNameInput && !machineNameInput.readOnly) {
        bindAutoSlug(labelInput, machineNameInput);
    }

    // Show/hide max_length field based on primitive_kind
    const primitiveKindSelect = document.getElementById('attr_primitive_kind');
    const maxLengthContainer = document.getElementById('max_length_container');

    if (primitiveKindSelect && maxLengthContainer) {
        primitiveKindSelect.addEventListener('change', () => {
            maxLengthContainer.classList.toggle('d-none', primitiveKindSelect.value !== 'TEXT');
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
                    // Plain text so the special default "now" can be typed.
                    defaultValueInput.type = 'text';
                    defaultValueInput.removeAttribute('step');
                    defaultValueInput.removeAttribute('min');
                    defaultValueInput.removeAttribute('max');
                    defaultValueInput.placeholder = 'now ou 2026-01-01T12:00 (vazio = NULL)';
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
            // Ensure default value input is enabled so it is included in the POST request,
            // but only if a type is selected.
            if (defaultValueInput && primitiveKindSelect && primitiveKindSelect.value) {
                defaultValueInput.disabled = false;
            }

            if (!form.checkValidity()) {
                event.preventDefault();
                event.stopPropagation();
            }
            form.classList.add('was-validated');
        }, false);
    }
});
