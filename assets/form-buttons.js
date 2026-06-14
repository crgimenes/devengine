/**
 * form-buttons.js
 * Framework for handling custom button actions in forms
 * Requires FormButtonActions to be defined before this script
 */

document.addEventListener('DOMContentLoaded', function () {
    // FormButtonActions will be defined by a separate per-form script
    if (typeof window.FormButtonActions === 'undefined') {
        window.FormButtonActions = {};
    }

    document.querySelectorAll('[data-form-action]').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            const actionName = this.dataset.formAction;
            const hasServerAction = this.dataset.hasServerAction === 'true';
            const confirmMsg = this.dataset.confirmMessage;
            const actionUrl = this.dataset.actionUrl;


            if (confirmMsg && !confirm(confirmMsg)) return;

            const form = this.closest('form');
            const formData = new FormData(form);

            // Run client JS first (if exists)
            if (window.FormButtonActions[actionName]) {
                const result = window.FormButtonActions[actionName](formData, this);
                if (result === false) return; // abort if returns false
            }

            // Then POST to server (if has server action)
            if (hasServerAction && actionUrl) {
                this.disabled = true;
                const originalContent = this.innerHTML;
                this.innerHTML = '<span class="spinner-border spinner-border-sm"></span> ...';

                fetch(actionUrl, {
                    method: 'POST',
                    body: formData
                })
                    .then(function (response) { return response.json(); })
                    .then(function (data) {
                        // Build redirect URL with message/error as query param
                        let redirectUrl = data.redirect_to || window.location.pathname;

                        if (data.error) {
                            redirectUrl += (redirectUrl.includes('?') ? '&' : '?') + 'error=' + encodeURIComponent(data.error);
                        } else if (data.message) {
                            redirectUrl += (redirectUrl.includes('?') ? '&' : '?') + 'message=' + encodeURIComponent(data.message);
                        }

                        window.location.href = redirectUrl;
                    })
                    .catch(function (err) {
                        // Show error in Bootstrap alert by reloading with error param
                        const redirectUrl = window.location.pathname + '?error=' + encodeURIComponent('Error: ' + err.message);
                        window.location.href = redirectUrl;
                    });
            }
        });
    });
});
