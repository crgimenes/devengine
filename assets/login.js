document.addEventListener('DOMContentLoaded', () => {
    const magicLinkForm = document.getElementById('magicLinkForm');
    if (magicLinkForm) {
        magicLinkForm.addEventListener('submit', async function (e) {
            e.preventDefault();
            const email = document.getElementById('email').value;
            const container = document.getElementById('messageContainer');

            try {
                const response = await fetch('/login/magic_link', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: 'email=' + encodeURIComponent(email)
                });

                if (response.ok) {
                    if (response.redirected) {
                        window.location.href = response.url;
                        return;
                    }

                    const contentType = response.headers.get("content-type");
                    if (contentType && contentType.includes("application/json")) {
                        const data = await response.json();
                        if (data.redirect) {
                            window.location.href = data.redirect;
                        }
                    } else {
                        // Successful response but not JSON and not a redirect (unlikely for Magic Link, but handled safe)
                        window.location.reload();
                    }
                } else {
                    container.innerHTML = `
                        <div class="alert alert-danger alert-dismissible fade show mt-3" role="alert">
                            Erro ao enviar link. Tente novamente.
                            <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
                        </div>
                    `;
                }
            } catch (err) {
                container.innerHTML = `
                    <div class="alert alert-danger alert-dismissible fade show mt-3" role="alert">
                        Erro na requisicao. Tente novamente.
                        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
                    </div>
                `;
            }
        });
    }
});
