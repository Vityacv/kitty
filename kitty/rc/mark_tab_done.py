#!/usr/bin/env python
# License: GPLv3 Copyright: 2025, Kovid Goyal <kovid at kovidgoyal.net>

from typing import TYPE_CHECKING

from .base import (
    MATCH_TAB_OPTION,
    ArgsType,
    Boss,
    PayloadGetType,
    PayloadType,
    RCOptions,
    RemoteCommand,
    ResponseType,
    Window,
)

if TYPE_CHECKING:
    from kitty.cli_stub import MarkTabDoneRCOptions as CLIOptions


class MarkTabDone(RemoteCommand):
    protocol_spec = __doc__ = '''
    match/str: The tab to mark as done
    '''

    short_desc = 'Mark a tab with a notification indicator'
    desc = (
        'Mark the specified tab(s) with a notification indicator. '
        'The indicator will be shown in select_tab overlay and cleared when the tab becomes active.'
    )
    options_spec = MATCH_TAB_OPTION
    argspec = ''

    def message_to_kitty(self, global_opts: RCOptions, opts: 'CLIOptions', args: ArgsType) -> PayloadType:
        return {'match': opts.match}

    def response_from_kitty(self, boss: Boss, window: Window | None, payload_get: PayloadGetType) -> ResponseType:
        from kitty.tabs import Tab
        for tab in self.tabs_for_match_payload(boss, window, payload_get):
            if isinstance(tab, Tab):
                # Only add notification if tab is not currently active
                if tab != boss.active_tab:
                    boss._tab_notifications.add(tab.id)
                    # Mark tab bar dirty to refresh display
                    tab.mark_tab_bar_dirty()
                    # Also mark all tab managers dirty to ensure all OS windows refresh
                    for tm in boss.all_tab_managers:
                        tm.mark_tab_bar_dirty()
        return None


mark_tab_done = MarkTabDone()
