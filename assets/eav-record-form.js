// EAV Record Form - Validation and UX
document.addEventListener('DOMContentLoaded', () => {
    const form = document.querySelector('.record-form');

    if (!form) {
        return;
    }

    // Bootstrap form validation
    form.addEventListener('submit', event => {
        if (!form.checkValidity()) {
            event.preventDefault();
            event.stopPropagation();
        }
        form.classList.add('was-validated');
    }, false);

    // Form dirty detection - warn on unsaved changes
    let formDirty = false;
    const inputs = form.querySelectorAll('input, select, textarea');

    inputs.forEach(input => {
        input.addEventListener('change', () => {
            formDirty = true;
        });
    });

    // Warn before leaving if form is dirty
    window.addEventListener('beforeunload', event => {
        if (formDirty) {
            event.preventDefault();
            event.returnValue = ''; // Required for Chrome
        }
    });

    // Don't warn if submitting
    form.addEventListener('submit', () => {
        formDirty = false;
    });

    // Type-specific validation
    const intInputs = form.querySelectorAll('input[type="number"][step="1"]');
    intInputs.forEach(input => {
        input.addEventListener('blur', () => {
            if (input.value && !Number.isInteger(parseFloat(input.value))) {
                input.setCustomValidity('Deve ser um número inteiro');
            } else {
                input.setCustomValidity('');
            }
        });
    });
});
