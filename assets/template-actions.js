document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('[data-confirm-message]').forEach(form => {
        form.addEventListener('submit', event => {
            if (!window.confirm(form.dataset.confirmMessage)) {
                event.preventDefault();
            }
        });
    });

    document.querySelectorAll('[data-select-on-click]').forEach(input => {
        input.addEventListener('click', () => input.select());
    });

    document.querySelectorAll('[data-submit-on-change]').forEach(control => {
        control.addEventListener('change', () => control.form?.requestSubmit());
    });
});
