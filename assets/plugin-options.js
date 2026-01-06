/**
 * plugin-options.js
 * Manages dynamic plugin options panel visibility and JSON generation
 */

document.addEventListener('DOMContentLoaded', function () {
    const kindSelect = document.getElementById('element_kind');
    const uiKindSelect = document.getElementById('ui_kind');
    const metaJsonField = document.getElementById('ui_meta_json');
    const optionsContainer = document.getElementById('plugin-options-container');

    if (!kindSelect || !optionsContainer) return;

    function showOptionsPanel(kind) {
        // Hide all panels
        document.querySelectorAll('.plugin-options-panel').forEach(function (panel) {
            panel.classList.add('d-none');
        });

        // Show the appropriate panel
        const panelId = 'options-' + kind;
        const panel = document.getElementById(panelId);
        if (panel) {
            panel.classList.remove('d-none');
        }

        // For field type, also show the appropriate ui_kind sub-panel
        if (kind === 'field' && uiKindSelect) {
            showFieldOptions(uiKindSelect.value);
        }
    }

    function showFieldOptions(uiKind) {
        // Hide all field ui options
        document.querySelectorAll('.field-ui-options').forEach(function (panel) {
            panel.classList.add('d-none');
        });

        // Show the appropriate field options panel
        const panelId = 'field-options-' + (uiKind || 'text');
        const panel = document.getElementById(panelId);
        if (panel) {
            panel.classList.remove('d-none');
        }
    }

    function updateJsonFromOptions() {
        const kind = kindSelect.value;
        const panelId = 'options-' + kind;
        const panel = document.getElementById(panelId);

        if (!panel) {
            return;
        }

        const options = {};

        // For field type, only collect from visible sub-panel
        let inputs;
        if (kind === 'field') {
            const visibleSubPanel = panel.querySelector('.field-ui-options:not(.d-none)');
            if (visibleSubPanel) {
                inputs = visibleSubPanel.querySelectorAll('.plugin-option');
            } else {
                inputs = [];
            }
        } else {
            inputs = panel.querySelectorAll('.plugin-option');
        }

        if (inputs) {
            inputs.forEach(function (input) {
                const key = input.dataset.key;
                if (!key) return;

                if (input.type === 'checkbox') {
                    options[key] = input.checked;
                } else if (input.type === 'number') {
                    const val = input.value.trim();
                    if (val !== '') {
                        options[key] = parseFloat(val);
                    }
                } else {
                    if (input.value.trim() !== '') {
                        options[key] = input.value;
                    }
                }
            });
        }

        // Only update if we have options
        if (Object.keys(options).length > 0) {
            metaJsonField.value = JSON.stringify(options, null, 2);
        } else {
            metaJsonField.value = '';
        }
    }

    function loadOptionsFromJson() {
        const kind = kindSelect.value;
        const panelId = 'options-' + kind;
        const panel = document.getElementById(panelId);

        if (!panel || !metaJsonField.value.trim()) {
            return;
        }

        try {
            const meta = JSON.parse(metaJsonField.value);
            const inputs = panel.querySelectorAll('.plugin-option');

            inputs.forEach(function (input) {
                const key = input.dataset.key;
                if (!key || !(key in meta)) return;

                if (input.type === 'checkbox') {
                    input.checked = !!meta[key];
                } else {
                    input.value = meta[key];
                }
            });
        } catch (e) {
            // Invalid JSON, ignore
        }
    }

    // Show initial panel based on current element kind
    showOptionsPanel(kindSelect.value);
    loadOptionsFromJson();

    // Update when element kind changes
    kindSelect.addEventListener('change', function () {
        showOptionsPanel(this.value);
        // Clear JSON when switching types
        metaJsonField.value = '';
    });

    // Update when ui_kind changes (for field type)
    if (uiKindSelect) {
        uiKindSelect.addEventListener('change', function () {
            if (kindSelect.value === 'field') {
                showFieldOptions(this.value);
                metaJsonField.value = '';
            }
        });
    }

    // Update JSON when any option changes
    optionsContainer.addEventListener('change', function (e) {
        if (e.target.classList.contains('plugin-option')) {
            updateJsonFromOptions();
        }
    });

    // Also listen for input events (for text/number fields)
    optionsContainer.addEventListener('input', function (e) {
        if (e.target.classList.contains('plugin-option')) {
            updateJsonFromOptions();
        }
    });
});
