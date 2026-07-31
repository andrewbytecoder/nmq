/* Highlight */
(function(hljs) {
    hljs.initHighlightingOnLoad();
})(hljs);

(function() {
    function normalizeText(value) {
        return (value || "").replace(/\s+/g, " ").trim();
    }

    function removeNode(node) {
        if (node && node.parentNode) {
            node.parentNode.removeChild(node);
        }
    }

    function removeMatchingTextNodes() {
        var selectors = "button, a, div, span";
        document.querySelectorAll(selectors).forEach(function(node) {
            var text = normalizeText(node.textContent);
            if (
                text === "Privacy Settings" ||
                text.includes("Privacy Settings") ||
                (text.includes("Privacy Policy") && text.includes("Accept"))
            ) {
                removeNode(node.closest("[role='dialog'], .ot-sdk-container, .ot-sdk-row, .ot-floating-button, .silktide-cookie-banner, .silktide-consent-manager, .slt-cmp-expose") || node);
            }
        });
    }

    function cleanupOnce() {
        document.querySelectorAll("header-app, .header-container__mobile").forEach(removeNode);
        document.querySelectorAll("header.md-header + div[style*='height:64px']").forEach(removeNode);
        document.querySelectorAll("#ot-sdk-btn-floating, .ot-floating-button, .ot-floating-button__front").forEach(removeNode);
        document.querySelectorAll("[id*='silktide' i], [class*='silktide' i], iframe[src*='silktide' i]").forEach(removeNode);
        removeMatchingTextNodes();

        var switcher = document.querySelector(".product-switcher .menu-item");
        if (switcher) {
            switcher.classList.add("ncp-static-label", "ncp-product-switcher__label");
            switcher.removeAttribute("href");
            switcher.removeAttribute("onclick");
            switcher.removeAttribute("aria-haspopup");
            switcher.setAttribute("aria-expanded", "false");
        }
    }

    function scheduleCleanup() {
        cleanupOnce();

        var attempts = 0;
        var maxAttempts = 10;
        var timer = window.setInterval(function() {
            cleanupOnce();
            attempts += 1;
            if (attempts >= maxAttempts) {
                window.clearInterval(timer);
            }
        }, 500);
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", scheduleCleanup, { once: true });
    } else {
        scheduleCleanup();
    }
})();
