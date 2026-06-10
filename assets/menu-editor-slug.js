// Auto-generate machine_name from the label field for the menu editor forms.
document.addEventListener('DOMContentLoaded', function () {
    bindAutoSlug(document.getElementById('label'), document.getElementById('machine_name'));
    bindAutoSlug(document.getElementById('new_label'), document.getElementById('new_machine_name'));
});
