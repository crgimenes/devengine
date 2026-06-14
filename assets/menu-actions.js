/**
 * menu-actions.js
 * Framework for handling custom menu item actions
 * Requires MenuItemActions to be defined before this script (via /menu/{menu}/actions.js)
 * Order of execution: Filo (server) -> JavaScript (client) -> Link (navigation)
 */

document.addEventListener('DOMContentLoaded', function () {
    // MenuItemActions will be defined by a separate per-menu script
    if (typeof window.MenuItemActions === 'undefined') {
        window.MenuItemActions = {};
    }

    document.querySelectorAll('[data-menu-action]').forEach(function (item) {
        item.addEventListener('click', function (e) {
            const actionName = this.dataset.menuAction;
            const hasFiloCode = this.dataset.hasFiloCode === 'true';
            const hasJsCode = this.dataset.hasJsCode === 'true';
            const actionUrl = this.dataset.actionUrl;
            const linkUrl = this.dataset.linkUrl;

            // If there's no action (only link), let the browser handle it naturally
            if (!hasFiloCode && !hasJsCode) {
                return; // Normal link behavior
            }

            e.preventDefault();

            // 1. Execute Filo code first (if present)
            if (hasFiloCode && actionUrl) {
                fetch(actionUrl, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    }
                })
                    .then(function (response) { return response.json(); })
                    .then(function (data) {
                        // Build redirect URL with message/error as query param (like form-buttons.js)
                        let redirectUrl = data.redirect_to || window.location.pathname;

                        if (data.error) {
                            // Show error in Bootstrap alert by reloading with error param
                            redirectUrl += (redirectUrl.includes('?') ? '&' : '?') + 'error=' + encodeURIComponent(data.error);
                            window.location.href = redirectUrl;
                            return;
                        }

                        if (data.message) {
                            redirectUrl += (redirectUrl.includes('?') ? '&' : '?') + 'message=' + encodeURIComponent(data.message);
                        }

                        // 2. Execute JS code (if present)
                        if (hasJsCode && window.MenuItemActions[actionName]) {
                            window.MenuItemActions[actionName]();
                        }

                        // 3. Navigate to link or reload with message
                        if (linkUrl) {
                            // If we have a message, append it to linkUrl
                            if (data.message && !redirectUrl.includes('message=')) {
                                let finalUrl = linkUrl;
                                finalUrl += (finalUrl.includes('?') ? '&' : '?') + 'message=' + encodeURIComponent(data.message);
                                window.location.href = finalUrl;
                            } else {
                                window.location.href = linkUrl;
                            }
                        } else if (data.message || data.redirect_to) {
                            // No link, but we have a message or redirect - go there
                            window.location.href = redirectUrl;
                        }
                        // If no link and no message/redirect, just stay on page after JS execution
                    })
                    .catch(function (err) {
                        // Show error in Bootstrap alert by reloading with error param
                        const redirectUrl = window.location.pathname + '?error=' + encodeURIComponent('Connection error: ' + err.message);
                        window.location.href = redirectUrl;
                    });
            } else {
                // No Filo code, just run JS and navigate

                // 2. Execute JS code (if present)
                if (hasJsCode && window.MenuItemActions[actionName]) {
                    window.MenuItemActions[actionName]();
                }

                // 3. Navigate to link (if present)
                if (linkUrl) {
                    window.location.href = linkUrl;
                }
            }
        });
    });
});
