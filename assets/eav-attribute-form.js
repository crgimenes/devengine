document.addEventListener('DOMContentLoaded', () => {
    const modal = document.getElementById('attributeModal');
    const form = modal?.querySelector('form');
    const modalTitle = modal?.querySelector('.modal-title');
    const machineNameInput = document.getElementById('attr_machine_name');
    const labelInput = document.getElementById('attr_label');
    const helpTextInput = document.getElementById('attr_help_text');
    const primitiveKindSelect = document.getElementById('attr_primitive_kind');
    const defaultValueInput = document.getElementById('attr_default_value');
    const isRequiredCheckbox = document.getElementById('attr_is_required');
    const isUniqueCheckbox = document.getElementById('attr_is_unique');
    const isIndexedCheckbox = document.getElementById('attr_is_indexed');
    const newAttrButton = document.querySelector('[data-bs-target="#attributeModal"]:not(.edit-attribute-btn)');

    let editMode = false;
    let currentAttrId = null;

    // Reset form for new attribute
    function resetFormForNew() {
        editMode = false;
        currentAttrId = null;

        if (modalTitle) modalTitle.textContent = 'Novo Atributo';

        const pathParts = window.location.pathname.split('/');
        const entityRefID = pathParts[pathParts.length - 2];
        form.action = `/tools/database-schema/eav/${entityRefID}/attributes/new`;

        form.reset();
        form.classList.remove('was-validated');

        if (defaultValueInput) {
            defaultValueInput.type = 'text';
            defaultValueInput.value = '';
            defaultValueInput.disabled = true;
            defaultValueInput.placeholder = 'Selecione um tipo primeiro';
        }
    }

    // Populate form for editing
    function populateFormForEdit(btn) {
        editMode = true;
        currentAttrId = btn.dataset.attrId;

        if (modalTitle) modalTitle.textContent = 'Editar Atributo';

        const pathParts = window.location.pathname.split('/');
        const entityRefID = pathParts[pathParts.length - 2];
        form.action = `/tools/database-schema/eav/${entityRefID}/attributes/${currentAttrId}/update`;

        if (machineNameInput) machineNameInput.value = btn.dataset.machineName || '';
        if (labelInput) labelInput.value = btn.dataset.label || '';
        if (helpTextInput) helpTextInput.value = btn.dataset.helpText || '';
        if (primitiveKindSelect) primitiveKindSelect.value = btn.dataset.primitiveKind || '';

        if (isRequiredCheckbox) isRequiredCheckbox.checked = btn.dataset.isRequired === 'true';
        if (isUniqueCheckbox) isUniqueCheckbox.checked = btn.dataset.isUnique === 'true';
        if (isIndexedCheckbox) isIndexedCheckbox.checked = btn.dataset.isIndexed === 'true';

        if (primitiveKindSelect) {
            const event = new Event('change');
            primitiveKindSelect.dispatchEvent(event);

            const kind = btn.dataset.primitiveKind;
            let defaultVal = '';

            switch (kind) {
                case 'BOOL':
                    defaultVal = btn.dataset.defaultBool || '0';
                    break;
                case 'INT':
                    defaultVal = btn.dataset.defaultInt || '';
                    break;
                case 'REAL':
                    defaultVal = btn.dataset.defaultReal || '';
                    break;
                case 'TEXT':
                    defaultVal = btn.dataset.defaultText || '';
                    break;
                case 'DATETIME':
                    defaultVal = btn.dataset.defaultDatetime || '';
                    break;
            }

            if (defaultValueInput) {
                defaultValueInput.value = defaultVal;
            }
        }
    }

    if (newAttrButton) {
        newAttrButton.addEventListener('click', () => {
            resetFormForNew();
        });
    }

    document.querySelectorAll('.edit-attribute-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            populateFormForEdit(btn);
        });
    });

    // Auto-generate machine_name from label on blur (only if empty)
    if (labelInput && machineNameInput) {
        labelInput.addEventListener('blur', () => {
            // Only auto-fill if machine_name is empty
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

    if (primitiveKindSelect && defaultValueInput) {
        primitiveKindSelect.addEventListener('change', () => {
            const kind = primitiveKindSelect.value;

            if (!kind) {
                defaultValueInput.type = 'text';
                defaultValueInput.value = '';
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
                    if (!editMode) defaultValueInput.value = '0';
                    defaultValueInput.placeholder = '0 (false) ou 1 (true)';
                    break;
                case 'INT':
                    defaultValueInput.type = 'number';
                    defaultValueInput.step = '1';
                    if (!editMode) defaultValueInput.value = '0';
                    defaultValueInput.placeholder = '0';
                    break;
                case 'REAL':
                    defaultValueInput.type = 'number';
                    defaultValueInput.step = 'any';
                    if (!editMode) defaultValueInput.value = '0.0';
                    defaultValueInput.placeholder = '0.0';
                    break;
                case 'TEXT':
                    defaultValueInput.type = 'text';
                    if (!editMode) defaultValueInput.value = '';
                    defaultValueInput.placeholder = 'String vazia';
                    break;
                case 'DATETIME':
                    defaultValueInput.type = 'datetime-local';
                    if (!editMode) defaultValueInput.value = '';
                    defaultValueInput.placeholder = 'NULL (deixe vazio para NULL)';
                    defaultValueInput.required = false;
                    break;
                default:
                    defaultValueInput.type = 'text';
                    defaultValueInput.value = '';
                    defaultValueInput.disabled = true;
                    defaultValueInput.placeholder = 'Selecione um tipo primeiro';
            }
        });
    }

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

    const deleteForms = document.querySelectorAll('.delete-attribute-form');
    deleteForms.forEach(form => {
        form.addEventListener('submit', event => {
            if (!confirm('Tem certeza que deseja excluir este atributo?')) {
                event.preventDefault();
            }
        });
    });
});
