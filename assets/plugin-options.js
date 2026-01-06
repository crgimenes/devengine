/**
 * plugin-options.js
 * Manages dynamic plugin options panel visibility and JSON generation
 */

document.addEventListener('DOMContentLoaded', function () {
    const kindSelect = document.getElementById('element_kind');
    const metaJsonField = document.getElementById('ui_meta_json');
    const optionsContainer = document.getElementById('plugin-options-container');

    if (!kindSelect || !optionsContainer) return;

    // Group element types that have their own options panels
    const groupTypes = ['group', 'accordion', 'card', 'tabs', 'carousel', 'field', 'divider'];

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
    }

    function updateJsonFromOptions() {
        const kind = kindSelect.value;
        const panelId = 'options-' + kind;
        const panel = document.getElementById(panelId);

        if (!panel) {
            return;
        }

        const options = {};
        const inputs = panel.querySelectorAll('.plugin-option');

        inputs.forEach(function (input) {
            const key = input.dataset.key;
            if (!key) return;

            if (input.type === 'checkbox') {
                options[key] = input.checked;
            } else if (input.type === 'number') {
                options[key] = parseInt(input.value, 10) || 0;
            } else {
                options[key] = input.value;
            }
        });

        // Only update if we have options
        if (Object.keys(options).length > 0) {
            metaJsonField.value = JSON.stringify(options, null, 2);
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

    // Update JSON when any option changes
    optionsContainer.addEventListener('change', function (e) {
        if (e.target.classList.contains('plugin-option')) {
            updateJsonFromOptions();
        }
    });

    // Also listen for input events (for number fields)
    optionsContainer.addEventListener('input', function (e) {
        if (e.target.classList.contains('plugin-option') && e.target.type === 'number') {
            updateJsonFromOptions();
        }
    });
});
