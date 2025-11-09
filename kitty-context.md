# Kitty Select Tab Enhancements - Context & Development Notes

## Use Case

**Primary workflow:** LLM session management for development
- Zed for code editing
- Kitty for multiple concurrent LLM sessions/experiments
- 10-20+ tabs representing different LLM contexts across projects
- Need rapid context switching with full context awareness

**Key constraints:**
- Multiple windows eat VRAM/RAM (GPU contexts × compositor resources)
- Single kitty window with tabs = minimal resource overhead
- Sway tabs don't provide rich context for navigation

## Current Implementation

Branch: `select-tab-enhancements`

### Features Implemented

**1. Rich Tab Display**
- App icons (emoji-based mapping)
- Idle time display (how long since tab was last active)
- Working directory display (cwd)
- Title truncation (configurable max length)
- Visual alignment with separators
- Automatic dimming of stale tabs (>1 day idle)

**2. Smart Caching**
- Cache /proc reads (exe, cwd) for instant overlay open
- Refresh cache on overlay close for next instant open
- ~0.5ms overhead eliminated, now instant
- Graceful handling of new tabs (cache miss ~0.05ms)

**3. Navigation Enhancements**
- Arrow key navigation (↑↓ to move selection)
- Page Up/Down (jump 3 items)
- Home/End (jump to first/last)
- Toggle behavior (open/close with same keybind)
- Hint-based selection still available

**4. Tab Deletion**
- Delete or Backspace key closes currently selected tab
- Dual backspace behavior: removes typed hints OR closes tab if no input typed
- Overlay automatically reopens after deletion for bulk operations
- Ghost tab filtering (skips `/proc/*/cwd` patterns from stale cache)

**5. Sorting Options** (`select_tab_sort_order`)
- `default` - Tab order
- `title` - Alphabetically by title
- `cwd` - Alphabetically by working directory
- `app` - Alphabetically by application name
- `mru` - Most recently used first
- `frequency` - Most frequently used first

Access tracking happens automatically in `set_active_tab()` using monotonic timestamps and counters.

**5. Configuration**
```conf
select_tab_sort_order mru          # or: default, title, cwd, app, frequency
select_tab_max_title_length 50     # 0 for unlimited
```

### Architecture

**Key Files:**
- `kitty/boss.py` - Main implementation
  - Lines 392-394: Cache and access tracking storage
  - Lines 2498-2508: Access tracking in `set_active_tab()`
  - Lines 3141-3311: `select_tab()` implementation
- `kitty/options/definition.py` - Configuration options
- `kittens/hints/main.go` - Arrow navigation

**Caching Strategy:**
- Boss-level cache: `_select_tab_cache: dict[int, tuple[str, str]]`
- Populate on first open (use cache if available, else read /proc)
- Refresh asynchronously on close for next instant open
- No TTL, no periodic refresh, no background timers

**Access Tracking:**
- `_tab_access_times: dict[int, float]` - Monotonic timestamps
- `_tab_access_counts: dict[int, int]` - Access counters
- Updated on every successful `set_active_tab()`

**Idle Time Display:**
- Uses existing `_tab_access_times` data (no additional storage needed)
- Format: `2h`, `45m`, `3d` for human-readable display
- Display order: `icon title │ idle │ cwd`
- Automatic dimming: Tabs idle >1 day shown in gray (\033[90m ANSI code)
- Time formatting helper: `format_idle_time()` in `select_tab()` method
- Calculated on-demand when overlay opens (no performance impact)

### Commits

1. `9b08815dc` - Enhance select_tab with icons, sorting, and performance optimizations
2. `fb06f1c03` - Add arrow navigation, toggle, and performance optimizations to select_tab
3. `92f5c8095` - Add instant select_tab with smart caching
4. `f3d4b57ba` - Add MRU and frequency sorting options to select_tab

## Lessons Learned

**The cwd cascade:**
- Added cwd display → needed /proc reads → added latency → built caching
- Added cwd display → long paths → needed title truncation
- "Created and solved our own problem" but cwd info is valuable for LLM session identification

**What's actually useful for upstream:**
- Arrow navigation
- Toggle behavior
- MRU/frequency sorting
- Maybe icons (though "student level design")

**What's workflow-specific:**
- cwd display + caching (solves problem we created)
- Title truncation (consequence of cwd)

## Philosophy: Upstream vs Mod

**Upstream select_tab:**
- Minimal, universal tool
- Works for everyone, stays out of the way
- Hint-based selection

**This mod:**
- Specialized for power users with 10-20+ tabs
- Tab dashboard with rich context
- Optimized for rapid context switching
- Usage-pattern aware (MRU/frequency)

**Transformation:** "Choose a tab" → "Tab workspace manager"

## Future Improvements

### High Impact

**1. Custom Persistent Tab Names**
- Beyond title, let user tag tabs: "Claude - project X", "GPT-4 - experiment Y"
- Store in boss-level dict, persist across restarts
- Show in select_tab instead of/alongside auto title
- **Impact:** Know exactly what each tab is without reading cwd

**2. Quick Duplicate Tab**
- Right-click context menu on tab with "Duplicate" option (more discoverable)
- Or `Ctrl+Shift+D` keybind to clone current tab+cwd
- Spawn new shell in same directory
- **Impact:** "New LLM session for same project" - context menu easier to remember than keybind
- **Note:** Requires tab bar mouse event handling and context menu implementation

**3. ~~Idle Time Display~~ ✓ IMPLEMENTED**
- ~~Track last activity per tab~~
- ~~Show in select_tab: "3h idle", "2d idle"~~
- ~~**Impact:** Know which sessions to close~~
- **Implemented:** Shows idle time for all tabs, dims tabs >1 day old

**4. Recently Closed Tabs** (lower priority)
- Track closed tabs (cwd, title, time)
- `Ctrl+Shift+T` to reopen last closed
- **Impact:** Recover accidentally closed LLM sessions
- **Note:** Low priority - no visual close button, accidental closes rare (requires Ctrl+Shift+W)

### Medium Impact

**5. Visual Groups/Separators**
- Group tabs by project/directory
- Show separators in select_tab overlay
- Manual or auto-group by cwd prefix

**6. Session Templates**
- Define templates: "New Claude session in ~/current-project"
- Quick spawn from select_tab or keybind

**7. Filter in select_tab**
- Type to filter tabs by name/cwd while overlay open
- Like fuzzy finder but for tabs
- (Note: User doesn't think fuzzy search needed with MRU sorting)

**8. Color-code tabs**
- Color tabs by project, tool, or manual tagging
- Visual distinction in tab bar and select_tab

### Lower Impact

**9. Context Preservation**
- Persist access times/counts across restarts
- Save custom tab names and metadata
- Session save/restore per project

**10. Cleanup Helpers**
- Bulk operations: "close all tabs for project X"
- Auto-suggest closing idle tabs
- Mark tabs as "temporary" vs "persistent"

**11. Model Name Detection**
- Parse process/env to detect LLM model in use
- Show in overlay: "Claude Sonnet 3.5", "GPT-4"

**12. Tab Preview**
- Show thumbnail or first few lines of active window
- Might be resource-heavy, conflicts with VRAM optimization goal

**13. Window Count per Tab**
- Show how many windows in each tab
- Useful for complex layouts

## Technical Patterns

**Performance:**
- Use monotonic timestamps (faster than datetime)
- Cache /proc reads aggressively
- Avoid O(n) operations in hot paths
- Pre-compute lowercase dicts for icon matching

**Cache invalidation:**
- Don't use TTL (stale data concerns)
- Don't use periodic timers (wasteful)
- Refresh on close = instant next open + fresh data

**Generator vs List:**
- `choose_entry` accepts `Iterable[tuple]`
- Generators work, but any output/logging breaks hints kitten
- Keep code between collection and display minimal

**Access tracking:**
- Track in `set_active_tab()` - central chokepoint
- Monotonic for timestamps (immune to clock changes)
- Simple counter for frequency

## Development Workflow

**Branch management:**
- Never commit to local `master` (stay clean for upstream pulls)
- Keep feature branches for each enhancement
- Optional: personal integration branch merging all features

**Sync with upstream:**
```bash
git fetch origin master
git rebase origin/master  # Apply your commits on top of new upstream
```

**Build:**
```bash
./dev.sh build --debug
# or with skip for faster iteration:
./dev.sh build --debug --skip-code-generation
```

**Config generation:**
```bash
./kitty/launcher/kitty +launch gen/config.py
```

## Resource Optimization Context

**Why single kitty window matters:**
- Each window = separate GPU context
- VRAM overhead: font atlases, glyph cache, rendering buffers
- Compositor overhead: window state, decorations, events
- With 20 windows: 20× overhead

**Current architecture:**
- Single kitty window = one GPU context
- Tabs share rendering resources
- Sway sees one window, minimal compositor load
- Trade navigation complexity for memory efficiency

**Additional benefits of single window:**
- **Window management complexity**: 20 kitty windows + browser + Zed + other apps = chaos
- Hard to find the right terminal among all sway windows
- `swaymsg -t get_tree` or rofi scripts still require search
- One kitty window = contained, predictable location in workspace
- **Crash risk**: Yes, single process crash = lose everything, but easier to manage than scattered windows
- Can use kitty sessions for persistence if needed

**Actual hierarchy:**
1. Sway workspaces = major context (personal/work/projects)
2. One kitty per workspace (or one fullscreen)
3. Kitty tabs = fine-grained LLM sessions
4. select_tab mod = makes #3 manageable

**Tab enhancements justify this trade-off:**
- Rich context makes navigation viable
- MRU/frequency reduces search time
- Instant open eliminates latency penalty
- Now competitive with multiple windows for UX
- Single point of failure risk < management overhead reduction

## Open Questions

1. **Persist metadata across restarts?**
   - Access times/counts currently ephemeral
   - Where to store? SQLite? JSON file?
   - Load on startup, save on close?

2. **Custom tab names - storage?**
   - Boss-level dict with tab_id → custom_name
   - Need persistence mechanism
   - Override vs supplement auto title?

3. **Idle time - definition?**
   - No keyboard/output in window?
   - No tab switches?
   - Track per-window or per-tab?

4. **Duplicate tab - behavior?**
   - Clone cwd only, or full layout?
   - Copy env vars?
   - New shell or fork process state?

5. **Groups - manual or auto?**
   - Manual tagging tedious but precise
   - Auto by cwd prefix might misgroup
   - Hybrid approach?

## Performance Notes

**Profiling impossibility:**
- ANY output (print, log_error_string, file write) breaks hints kitten
- "object of type 'generator' has no len()" error
- External timing only

**Measured latency (before caching):**
- ~0.5ms total for select_tab open
- ~0.05ms per /proc read (exe + cwd)
- Icon loading pre-cached in Boss: negligible

**After caching:**
- First open: ~0.5ms (cache miss)
- Subsequent opens: instant (cache hit)
- New tab cache miss: ~0.05ms overhead

## Notes

- Icons are "student level design" - basic emoji mapping
- Cwd info valuable for LLM session identification despite perf cost
- Arrow navigation more useful than expected for nearby tabs
- MRU sorting might eliminate need for fuzzy search
- Toggle behavior feels natural, reduces cognitive load
