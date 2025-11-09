#!/usr/bin/env python
# License: GPLv3 Copyright: 2024, Codex

from __future__ import annotations

import json
from typing import TYPE_CHECKING

from kitty.fast_data_types import font_cache_stats

from .base import ArgsType, Boss, PayloadGetType, PayloadType, RCOptions, RemoteCommand, ResponseType, Window

if TYPE_CHECKING:
    from kitty.cli_stub import RCOptions as CLIOptions  # pragma: no cover


class CacheStats(RemoteCommand):

    short_desc = 'Report font atlas cache statistics'
    desc = (
        'Return JSON describing the glyph atlas caches Kitty has built so far. '
        'Includes per-font-group sprite counts and approximate GPU memory usage to help track VRAM growth.'
    )

    def message_to_kitty(self, global_opts: RCOptions, opts: RCOptions, args: ArgsType) -> PayloadType:
        return {}

    def response_from_kitty(self, boss: Boss, window: Window | None, payload_get: PayloadGetType) -> ResponseType:
        stats = font_cache_stats()
        return json.dumps(stats, indent=2, sort_keys=True)


cache_stats = CacheStats()
