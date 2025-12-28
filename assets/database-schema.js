// Database Schema - EAV Table Row Click Handlers
document.addEventListener('DOMContentLoaded', () => {
    // Make EAV table rows clickable to view records
    document.querySelectorAll('.eav-table-row').forEach(row => {
        const recordsUrl = row.dataset.recordsUrl;
        if (!recordsUrl) return;

        row.addEventListener('click', (e) => {
            // Don't navigate if clicking on a button or link
            if (e.target.closest('.eav-edit-btn, .btn-group')) {
                return;
            }
            window.location.href = recordsUrl;
        });
    });
});
