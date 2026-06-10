// Auto-generate machine_name from the name field.
document.addEventListener('DOMContentLoaded', function () {
    bindAutoSlug(document.getElementById('name'), document.getElementById('machine_name'));

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
});
