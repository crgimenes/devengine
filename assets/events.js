// Minimalist SSE client.
(function () {
  let es = null;

  try {
    es = new EventSource("/events");

    es.onmessage = function (event) {
      const msg = String(event.data || "");

      // Ignore heartbeat
      if (msg.startsWith("heartbeat") || msg === "") {
        return;
      }

      // Handle refresh message
      if (msg === "refresh") {
        location.reload();
      }
    };

    es.onerror = function () {
      // EventSource retries automatically; nothing to do.
    };
  } catch (err) {
    console.error("[SSE] Error:", err);
  }

  // Cleanup on page unload
  window.addEventListener("beforeunload", function () {
    if (es) {
      es.close();
    }
  });
})();
