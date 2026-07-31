from __future__ import annotations

import re


FONT_PRECONNECT_RE = re.compile(
    r"""\s*<link\s+href="https://fonts\.gstatic\.com"\s+rel="preconnect"\s+crossorigin>\s*""",
    re.IGNORECASE,
)

FONT_STYLESHEET_RE = re.compile(
    r"""\s*<link\s+rel="stylesheet"\s+href="https://fonts\.googleapis\.com/css\?family=Roboto:[^"]*">\s*""",
    re.IGNORECASE,
)

FONT_INLINE_STYLE_RE = re.compile(
    r"""\s*<style>body,input\{font-family:"Roboto"[^<]*?code,kbd,pre\{font-family:"Roboto Mono"[^<]*?</style>\s*""",
    re.IGNORECASE | re.DOTALL,
)

LOCAL_FONT_STYLE = """
<style>
body,
input,
button,
select,
textarea {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC",
    "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", Helvetica, Arial,
    sans-serif;
}

code,
kbd,
pre {
  font-family: "Cascadia Mono", "SFMono-Regular", Consolas, "Liberation Mono",
    Menlo, monospace;
}
</style>
"""


def on_post_page(output: str, /, **kwargs) -> str:
    updated = FONT_PRECONNECT_RE.sub("", output)
    updated = FONT_STYLESHEET_RE.sub("", updated)
    updated = FONT_INLINE_STYLE_RE.sub(LOCAL_FONT_STYLE, updated)
    return updated
