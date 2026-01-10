document.addEventListener('DOMContentLoaded', function() {
    const toggleBtn = document.getElementById('mobile-menu-toggle');
    const closeBtn = document.getElementById('mobile-menu-close');
    const backdrop = document.getElementById('mobile-backdrop');
    
    function toggleMenu() {
        const menu = document.getElementById('mobile-menu');
        const backdrop = document.getElementById('mobile-backdrop');
        document.body.classList.toggle('overflow-hidden');
        menu.classList.toggle('active');
        backdrop.classList.toggle('active');
    }

    if (toggleBtn) {
        toggleBtn.addEventListener('click', toggleMenu);
    }

    if (closeBtn) {
        closeBtn.addEventListener('click', toggleMenu);
    }

    if (backdrop) {
        backdrop.addEventListener('click', toggleMenu);
    }
});
