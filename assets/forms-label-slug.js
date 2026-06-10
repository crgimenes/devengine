// Auto-generate machine_name from the label field.
document.addEventListener('DOMContentLoaded', function () {
    bindAutoSlug(document.getElementById('label'), document.getElementById('machine_name'));
});
