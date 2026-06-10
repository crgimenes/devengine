// Shared slug helper for machine_name fields.
//
// slugify(text) turns a human label into a safe machine identifier.
// bindAutoSlug(labelEl, machineEl) keeps machine_name in sync with the label
// while the user types, until the user edits machine_name by hand. Editing
// the label after that no longer overwrites the manual value.

function slugify(text) {
    return text
        .toString()
        .toLowerCase()
        .trim()
        .normalize('NFD')
        .replace(/\p{Diacritic}/gu, '')
        .replace(/[^a-z0-9\s_]/g, '')
        .replace(/\s+/g, '_')
        .replace(/_+/g, '_')
        .replace(/^_|_$/g, '');
}

function bindAutoSlug(labelEl, machineEl) {
    if (!labelEl || !machineEl) {
        return;
    }

    // A field that arrives non-empty (edit forms) counts as manually set.
    let dirty = machineEl.value.trim() !== '';

    labelEl.addEventListener('input', function () {
        if (dirty) {
            return;
        }
        machineEl.value = slugify(labelEl.value);
    });

    machineEl.addEventListener('input', function () {
        dirty = machineEl.value.trim() !== '';
    });
}

window.slugify = slugify;
window.bindAutoSlug = bindAutoSlug;
