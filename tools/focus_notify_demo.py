#!/usr/bin/env python3
"""
Focus-aware notification demo for terminal applications.

Run inside a focus-reporting terminal (e.g. kitty, GNOME Terminal, iTerm2).
The script enables focus tracking and triggers `notify-send` every five seconds
only when the terminal window is not focused (or when focus tracking is
unsupported).
"""

from __future__ import annotations

import atexit
import os
import selectors
import subprocess
import sys
import termios
import time
import tty

FOCUS_IN = b"\x1b[I"
FOCUS_OUT = b"\x1b[O"
FOCUS_ENABLE = "\x1b[?1004h"
FOCUS_DISABLE = "\x1b[?1004l"


def main() -> None:
    fd = sys.stdin.fileno()
    old_term = termios.tcgetattr(fd)

    def restore_terminal() -> None:
        """Reset raw mode and disable focus tracking on exit."""
        termios.tcsetattr(fd, termios.TCSADRAIN, old_term)
        sys.stdout.write(FOCUS_DISABLE)
        sys.stdout.flush()

    atexit.register(restore_terminal)

    tty.setcbreak(fd)
    sys.stdout.write(FOCUS_ENABLE)
    sys.stdout.flush()

    selector = selectors.DefaultSelector()
    selector.register(sys.stdin, selectors.EVENT_READ)

    buffer = b""
    focused = True
    supported = False
    interval = 5.0
    next_ping = time.monotonic() + interval

    print(
        "Focus demo running. Switch to another window to allow notifications.\n"
        "Press Ctrl+C to stop."
    )

    try:
        while True:
            timeout = max(0.0, next_ping - time.monotonic())
            events = selector.select(timeout)

            if events:
                chunk = os.read(fd, 32)
                buffer += chunk

                while True:
                    if buffer.startswith(FOCUS_IN):
                        focused = True
                        supported = True
                        buffer = buffer[len(FOCUS_IN) :]
                    elif buffer.startswith(FOCUS_OUT):
                        focused = False
                        supported = True
                        buffer = buffer[len(FOCUS_OUT) :]
                    elif buffer.startswith(b"\x1b["):
                        # Skip over other CSI sequences quickly.
                        pos = buffer.find(b"m")
                        if pos == -1:
                            break
                        buffer = buffer[pos + 1 :]
                    elif buffer:
                        # Keep only the tail so we can match future sequences.
                        buffer = buffer[-2:]
                        break
                    else:
                        break

            now = time.monotonic()
            if now >= next_ping:
                if focused and supported:
                    print("Terminal focused; skipping notify-send.")
                else:
                    try:
                        subprocess.run(
                            ["notify-send", "Focus demo", "Terminal not focused."],
                            check=False,
                        )
                        print("Dispatched notify-send.")
                    except FileNotFoundError:
                        print("notify-send not found; exiting.")
                        return

                next_ping = now + interval

    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
