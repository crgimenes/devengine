// Auto-fill form element fields from the selected EAV attribute, and keep
// machine_name in sync with the label.
document.addEventListener('DOMContentLoaded', function () {
    const eavSelect = document.getElementById('eav_attribute_select');
    const labelInput = document.getElementById('element_label');
    const machineNameInput = document.getElementById('element_machine_name');

    if (!labelInput || !machineNameInput) {
        return;
    }

    bindAutoSlug(labelInput, machineNameInput);

    if (eavSelect) {
        eavSelect.addEventListener('change', function () {
            const selectedOption = this.options[this.selectedIndex];
            const attrLabel = selectedOption.dataset.label || '';
            const attrMachine = selectedOption.dataset.machine || '';

            if (labelInput.value.trim() === '' && attrLabel) {
                labelInput.value = attrLabel;
            }
            if (machineNameInput.value.trim() === '' && attrMachine) {
                machineNameInput.value = attrMachine;
            }
        });
    }
});
