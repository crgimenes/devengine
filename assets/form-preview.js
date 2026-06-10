// form-preview.js
//
// Sends the current form values to /form/<machine>/preview as the user types
// and writes the returned computed values back into the matching inputs.
// Bound automatically to any <form data-form-machine="...">.

(function () {
  'use strict';

  const DEBOUNCE_MS = 300;

  function setupForm(form) {
    const machine = form.dataset.formMachine;
    if (!machine) return;

    let timer = null;
    let inFlight = false;

    function schedule() {
      if (timer) clearTimeout(timer);
      timer = setTimeout(run, DEBOUNCE_MS);
    }

    function run() {
      if (inFlight) {
        schedule();
        return;
      }
      inFlight = true;
      const data = new FormData(form);
      fetch('/form/' + encodeURIComponent(machine) + '/preview', {
        method: 'POST',
        body: data,
        credentials: 'same-origin',
        headers: { 'Accept': 'application/json' },
      })
        .then(function (resp) {
          if (!resp.ok) throw new Error('preview status ' + resp.status);
          return resp.json();
        })
        .then(function (out) {
          Object.keys(out || {}).forEach(function (name) {
            const input = form.querySelector('[name="' + cssEscape(name) + '"]');
            if (!input) return;
            const v = out[name];
            input.value = v === null || v === undefined ? '' : String(v);
          });
        })
        .catch(function (err) {
          // Silent: preview is best-effort.
          if (window.console) console.warn('preview:', err);
        })
        .finally(function () {
          inFlight = false;
        });
    }

    form.addEventListener('input', function (e) {
      const t = e.target;
      if (!t || !t.name) return;
      if (t.matches('input[readonly], textarea[readonly]')) return;
      schedule();
    });
    form.addEventListener('change', function (e) {
      const t = e.target;
      if (!t || !t.name) return;
      schedule();
    });
  }

  // Minimal CSS.escape polyfill — older targets without CSS.escape get one.
  function cssEscape(s) {
    if (window.CSS && window.CSS.escape) return window.CSS.escape(s);
    return String(s).replace(/[^a-zA-Z0-9_-]/g, function (ch) {
      return '\\' + ch;
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('form[data-form-machine]').forEach(setupForm);
  });
})();
