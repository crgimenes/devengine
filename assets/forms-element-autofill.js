// Auto-fill form element fields from EAV attribute selection
document.addEventListener('DOMContentLoaded', function () {
    const eavSelect = document.getElementById('eav_attribute_select');
    const labelInput = document.getElementById('element_label');
    const machineNameInput = document.getElementById('element_machine_name');

    if (!eavSelect || !labelInput || !machineNameInput) {
        return; // Elements not found, exit gracefully
    }

    // When EAV attribute is selected, auto-fill label and machine_name
    eavSelect.addEventListener('change', function () {
        const selectedOption = this.options[this.selectedIndex];
        const attrLabel = selectedOption.dataset.label || '';
        const attrMachine = selectedOption.dataset.machine || '';

        // Only fill if the fields are empty
        if (labelInput.value.trim() === '' && attrLabel) {
            labelInput.value = attrLabel;
        }
        if (machineNameInput.value.trim() === '' && attrMachine) {
            machineNameInput.value = attrMachine;
        }
    });

    // Auto-generate machine_name from label on blur
    labelInput.addEventListener('blur', function () {
        if (machineNameInput.value.trim() === '' && this.value.trim() !== '') {
            machineNameInput.value = slugify(this.value);
        }
    });

    function slugify(text) {
        return text
            .toString()
            .toLowerCase()
            .trim()
            .normalize('NFD')
            .replace(/[\u0300-\u036f]/g, '')
            .replace(/[^a-z0-9\s_]/g, '')
            .replace(/\s+/g, '_')
            .replace(/_+/g, '_')
            .replace(/^_|_$/g, '');
    }
});
