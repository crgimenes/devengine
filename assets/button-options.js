/**
 * button-options.js
 * Manages visibility of button-specific options based on element_kind selection
 */

document.addEventListener('DOMContentLoaded', function () {
    const kindSelect = document.getElementById('element_kind');
    const buttonOpts = document.getElementById('button_options');

    if (!kindSelect || !buttonOpts) {
        return;
    }

    function updateButtonOptionsVisibility() {
        if (kindSelect.value === 'button') {
            buttonOpts.classList.remove('d-none');
        } else {
            buttonOpts.classList.add('d-none');
        }
    }

    // Run on page load
    updateButtonOptionsVisibility();

    // Run on change
    kindSelect.addEventListener('change', updateButtonOptionsVisibility);
});
