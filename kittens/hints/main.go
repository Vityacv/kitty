// License: GPLv3 Copyright: 2023, Kovid Goyal, <kovid at kovidgoyal.net>

package hints

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/kovidgoyal/kitty/tools/cli"
	"github.com/kovidgoyal/kitty/tools/tty"
	"github.com/kovidgoyal/kitty/tools/tui"
	"github.com/kovidgoyal/kitty/tools/tui/loop"
	"github.com/kovidgoyal/kitty/tools/utils"
	"github.com/kovidgoyal/kitty/tools/utils/style"
	"github.com/kovidgoyal/kitty/tools/wcswidth"
)

var _ = fmt.Print

func convert_text(text string, cols int) string {
	lines := make([]string, 0, 64)
	empty_line := strings.Repeat("\x00", cols) + "\n"
	s1 := utils.NewLineScanner(text)
	for s1.Scan() {
		full_line := s1.Text()
		if full_line == "" {
			lines = append(lines, empty_line)
			continue
		}
		if strings.TrimRight(full_line, "\r") == "" {
			for range len(full_line) {
				lines = append(lines, empty_line)
			}
			continue
		}
		appended := false
		s2 := utils.NewSeparatorScanner(full_line, "\r")
		for s2.Scan() {
			line := s2.Text()
			if line != "" {
				line_sz := wcswidth.Stringwidth(line)
				extra := cols - line_sz
				if extra > 0 {
					line += strings.Repeat("\x00", extra)
				}
				lines = append(lines, line)
				lines = append(lines, "\r")
				appended = true
			}
		}
		if appended {
			lines[len(lines)-1] = "\n"
		}
	}
	ans := strings.Join(lines, "")
	return strings.TrimRight(ans, "\r\n")
}

func parse_input(text string) string {
	cols, err := strconv.Atoi(os.Getenv("OVERLAID_WINDOW_COLS"))
	if err == nil {
		return convert_text(text, cols)
	}
	term, err := tty.OpenControllingTerm()
	if err == nil {
		sz, err := term.GetSize()
		term.Close()
		if err == nil {
			return convert_text(text, int(sz.Col))
		}
	}
	return convert_text(text, 80)
}

type Result struct {
	Match                []string         `json:"match"`
	Programs             []string         `json:"programs"`
	Multiple_joiner      string           `json:"multiple_joiner"`
	Customize_processing string           `json:"customize_processing"`
	Type                 string           `json:"type"`
	Groupdicts           []map[string]any `json:"groupdicts"`
	Extra_cli_args       []string         `json:"extra_cli_args"`
	Linenum_action       string           `json:"linenum_action"`
	Cwd                  string           `json:"cwd"`
}

func encode_hint(num int, alphabet string) (res string) {
	runes := []rune(alphabet)
	d := len(runes)
	for res == "" || num > 0 {
		res = string(runes[num%d]) + res
		num /= d
	}
	return
}

func decode_hint(x string, alphabet string) (ans int) {
	base := len(alphabet)
	index_map := make(map[rune]int, len(alphabet))
	for i, c := range alphabet {
		index_map[c] = i
	}
	for _, char := range x {
		ans = ans*base + index_map[char]
	}
	return
}

func as_rgb(c uint32) [3]float32 {
	return [3]float32{float32((c>>16)&255) / 255.0, float32((c>>8)&255) / 255.0, float32(c&255) / 255.0}
}

func hints_text_color(confval string) (ans string) {
	ans = confval
	if ans == "auto" {
		ans = "bright-gray"
		if bc, err := tui.ReadBasicColors(); err == nil {
			bg := as_rgb(bc.Background)
			c15 := as_rgb(bc.Color15)
			c8 := as_rgb(bc.Color8)
			if utils.RGBContrast(bg[0], bg[1], bg[2], c8[0], c8[1], c8[2]) > utils.RGBContrast(bg[0], bg[1], bg[2], c15[0], c15[1], c15[2]) {
				ans = "bright-black"
			}
		}
	}
	return
}

func main(_ *cli.Command, o *Options, args []string) (rc int, err error) {
	o.HintsTextColor = hints_text_color(o.HintsTextColor)
	output := tui.KittenOutputSerializer()
	if tty.IsTerminal(os.Stdin.Fd()) {
		return 1, fmt.Errorf("You must pass the text to be hinted on STDIN")
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1, fmt.Errorf("Failed to read from STDIN with error: %w", err)
	}
	if len(args) > 0 && o.CustomizeProcessing == "" && o.Type != "linenum" {
		return 1, fmt.Errorf("Extra command line arguments present: %s", strings.Join(args, " "))
	}
	input_text := parse_input(utils.UnsafeBytesToString(stdin))
	text, all_marks, index_map, err := find_marks(input_text, o, os.Args[2:]...)
	if err != nil {
		return 1, err
	}

	result := Result{
		Programs: o.Program, Multiple_joiner: o.MultipleJoiner, Customize_processing: o.CustomizeProcessing, Type: o.Type,
		Extra_cli_args: args, Linenum_action: o.LinenumAction,
	}
	result.Cwd, _ = os.Getwd()
	alphabet := o.Alphabet
	if alphabet == "" {
		alphabet = DEFAULT_HINT_ALPHABET
	}
	ignore_mark_indices := utils.NewSet[int](8)
	window_title := o.WindowTitle
	if window_title == "" {
		switch o.Type {
		case "url":
			window_title = "Choose URL"
		default:
			window_title = "Choose text"
		}
	}
	current_text := ""
	current_input := ""
	match_suffix := ""
	switch o.AddTrailingSpace {
	case "always":
		match_suffix = " "
	case "never":
	default:
		if o.Multiple {
			match_suffix = " "
		}
	}
	chosen := []*Mark{}
	lp, err := loop.New(loop.NoAlternateScreen) // no alternate screen reduces flicker on exit
	if err != nil {
		return
	}
	fctx := style.Context{AllowEscapeCodes: true}
	faint := fctx.SprintFunc("dim")
	hint_style := fctx.SprintFunc(fmt.Sprintf("fg=%s bg=%s bold", o.HintsForegroundColor, o.HintsBackgroundColor))
	text_style := fctx.SprintFunc(fmt.Sprintf("fg=%s bold", o.HintsTextColor))
	selected_style := fctx.SprintFunc("bg=#444444 bold") // Highlight selected item with gray background

	// Build ordered list of indices for arrow navigation (sorted by position in text, not by index)
	// This respects the visual order of tabs as displayed (which follows select_tab_sort_order)
	type indexWithPos struct {
		idx int
		pos int
	}
	indices_with_pos := make([]indexWithPos, 0, len(index_map))
	for idx, m := range index_map {
		indices_with_pos = append(indices_with_pos, indexWithPos{idx: idx, pos: m.Start})
	}
	sort.Slice(indices_with_pos, func(i, j int) bool {
		return indices_with_pos[i].pos < indices_with_pos[j].pos
	})
	ordered_indices := make([]int, len(indices_with_pos))
	for i, item := range indices_with_pos {
		ordered_indices[i] = item.idx
	}

	// Track position in ordered list for arrow navigation
	selected_position := -1
	// Find initial selected position (line with ◄ marker)
	for pos, idx := range ordered_indices {
		m := index_map[idx]
		mark_text := text[m.Start:m.End]
		if strings.Contains(mark_text, "◄") {
			selected_position = pos
			break
		}
	}
	if selected_position == -1 && len(ordered_indices) > 0 {
		selected_position = 0 // Default to first item if no ◄ found
	}

	get_selected_index := func() int {
		if selected_position >= 0 && selected_position < len(ordered_indices) {
			return ordered_indices[selected_position]
		}
		return -1
	}

	highlight_mark := func(m *Mark, mark_text string) string {
		hint := encode_hint(m.Index, alphabet)
		if current_input != "" && !strings.HasPrefix(hint, current_input) {
			return faint(mark_text)
		}
		hint = hint[len(current_input):]
		if hint == "" {
			hint = " "
		}
		if len(mark_text) <= len(hint) {
			mark_text = ""
		} else {
			replaced_text := mark_text[:len(hint)]
			replaced_text = strings.ReplaceAll(replaced_text, "\r", "\n")
			if strings.Contains(replaced_text, "\n") {
				buf := strings.Builder{}
				buf.Grow(2 * len(hint))
				h := hint
				parts := strings.Split(replaced_text, "\n")
				for i, x := range parts {
					if x != "" {
						buf.WriteString(h[:len(x)])
						h = h[len(x):]
					}
					if i != len(parts)-1 {
						buf.WriteString("\n")
					}
				}
				if h != "" {
					buf.WriteString(h)
				}
				hint = buf.String()
			}
			mark_text = mark_text[len(hint):]
		}

		// Apply selected highlighting if this is the keyboard-selected item
		var ans string
		if m.Index == get_selected_index() {
			ans = selected_style(hint) + selected_style(mark_text)
		} else {
			ans = hint_style(hint) + text_style(mark_text)
		}
		return fmt.Sprintf("\x1b]8;;mark:%d\a%s\x1b]8;;\a", m.Index, ans)
	}

	render := func() string {
		ans := text
		for i := len(all_marks) - 1; i >= 0; i-- {
			mark := &all_marks[i]
			if ignore_mark_indices.Has(mark.Index) {
				continue
			}
			mtext := highlight_mark(mark, ans[mark.Start:mark.End])
			ans = ans[:mark.Start] + mtext + ans[mark.End:]
		}
		ans = strings.ReplaceAll(ans, "\x00", "")
		return strings.TrimRightFunc(strings.NewReplacer("\r", "\r\n", "\n", "\r\n").Replace(ans), unicode.IsSpace)
	}

	draw_screen := func() {
		lp.StartAtomicUpdate()
		defer lp.EndAtomicUpdate()
		if current_text == "" {
			current_text = render()
		}
		lp.ClearScreen()
		lp.QueueWriteString(current_text)
	}
	reset := func() {
		current_input = ""
		current_text = ""
	}

	lp.OnInitialize = func() (string, error) {
		lp.SetCursorVisible(false)
		lp.SetWindowTitle(window_title)
		lp.AllowLineWrapping(false)
		lp.MouseTrackingMode(loop.BUTTONS_ONLY_MOUSE_TRACKING)
		draw_screen()
		lp.SendOverlayReady()
		return "", nil
	}
	lp.OnFinalize = func() string {
		lp.SetCursorVisible(true)
		return ""
	}
	lp.OnResize = func(old_size, new_size loop.ScreenSize) error {
		draw_screen()
		return nil
	}
	// Handle right-click for closing tabs in select_tab
	right_click_mode := false
	lp.OnMouseEvent = func(ev *loop.MouseEvent) error {
		if ev.Event_type == loop.MOUSE_RELEASE && ev.Buttons&loop.RIGHT_MOUSE_BUTTON != 0 {
			// Right-click released - set flag, hyperlink click will follow
			right_click_mode = true
			return nil
		}
		if ev.Event_type == loop.MOUSE_PRESS && ev.Buttons&loop.RIGHT_MOUSE_BUTTON != 0 {
			// Right-click pressed - set flag for close action
			right_click_mode = true
		} else if ev.Event_type == loop.MOUSE_PRESS {
			// Any other button pressed - clear flag
			right_click_mode = false
		}
		return nil
	}

	lp.OnRCResponse = func(data []byte) error {
		var r struct {
			Type string
			Mark int
		}
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		if r.Type == "mark_activated" {
			if m, ok := index_map[r.Mark]; ok {
				if right_click_mode {
					// Right-click on hyperlink - signal close action
					// Use negative index to indicate close instead of select
					m_copy := *m
					m_copy.Index = -m.Index - 1 // Make negative and offset by 1 to differentiate from -0
					chosen = append(chosen, &m_copy)
					right_click_mode = false
				} else {
					// Regular left-click
					chosen = append(chosen, m)
				}
				if o.Multiple {
					ignore_mark_indices.Add(m.Index)
					reset()
				} else {
					lp.Quit(0)
					return nil
				}

			}
		}
		return nil
	}

	lp.OnText = func(text string, _, _ bool) error {
		changed := false
		for _, ch := range text {
			if strings.ContainsRune(alphabet, ch) {
				test_input := current_input + string(ch)
				// Check if this input would match any valid hint
				has_match := false
				for idx := range index_map {
					if eh := encode_hint(idx, alphabet); strings.HasPrefix(eh, test_input) {
						has_match = true
						break
					}
				}
				if !has_match {
					// No valid hint starts with this input, quit like ESC
					if o.Multiple {
						lp.Quit(0)
					} else {
						lp.Quit(1)
					}
					return nil
				}
				current_input = test_input
				changed = true
			}
		}
		if changed {
			matches := []*Mark{}
			for idx, m := range index_map {
				if eh := encode_hint(idx, alphabet); strings.HasPrefix(eh, current_input) {
					matches = append(matches, m)
				}
			}
			if len(matches) == 1 {
				chosen = append(chosen, matches[0])
				if o.Multiple {
					ignore_mark_indices.Add(matches[0].Index)
					reset()
				} else {
					lp.Quit(0)
					return nil
				}
			}
			current_text = ""
			draw_screen()
		}
		return nil
	}

	lp.OnKeyEvent = func(ev *loop.KeyEvent) error {
		if ev.MatchesPressOrRepeat("backspace") {
			ev.Handled = true
			r := []rune(current_input)
			if len(r) > 0 {
				// If there's typed input, remove last character
				r = r[:len(r)-1]
				current_input = string(r)
				current_text = ""
				draw_screen()
			} else {
				// If no typed input, close selected tab
				idx := get_selected_index()
				if idx >= 0 {
					if m := index_map[idx]; m != nil {
						// Mark this as a close action
						m_copy := *m
						if m_copy.Groupdict == nil {
							m_copy.Groupdict = make(map[string]any)
						}
						m_copy.Groupdict["close_action"] = true
						chosen = append(chosen, &m_copy)
						lp.Quit(0)
					}
				}
			}
		} else if ev.MatchesPressOrRepeat("down") || ev.MatchesPressOrRepeat("tab") {
			ev.Handled = true
			// Move selection down (next item)
			if len(ordered_indices) > 0 {
				selected_position++
				if selected_position >= len(ordered_indices) {
					selected_position = 0 // Wrap to first
				}
				current_text = ""
				draw_screen()
			}
		} else if ev.MatchesPressOrRepeat("up") || ev.MatchesPressOrRepeat("shift+tab") {
			ev.Handled = true
			// Move selection up (previous item)
			if len(ordered_indices) > 0 {
				selected_position--
				if selected_position < 0 {
					selected_position = len(ordered_indices) - 1 // Wrap to last
				}
				current_text = ""
				draw_screen()
			}
		} else if ev.MatchesPressOrRepeat("page_down") {
			ev.Handled = true
			// Jump down 3 items
			if len(ordered_indices) > 0 {
				selected_position += 3
				if selected_position >= len(ordered_indices) {
					selected_position = len(ordered_indices) - 1 // Stop at last
				}
				current_text = ""
				draw_screen()
			}
		} else if ev.MatchesPressOrRepeat("page_up") {
			ev.Handled = true
			// Jump up 3 items
			if len(ordered_indices) > 0 {
				selected_position -= 3
				if selected_position < 0 {
					selected_position = 0 // Stop at first
				}
				current_text = ""
				draw_screen()
			}
		} else if ev.MatchesPressOrRepeat("home") {
			ev.Handled = true
			// Jump to first item
			if len(ordered_indices) > 0 {
				selected_position = 0
				current_text = ""
				draw_screen()
			}
		} else if ev.MatchesPressOrRepeat("end") {
			ev.Handled = true
			// Jump to last item
			if len(ordered_indices) > 0 {
				selected_position = len(ordered_indices) - 1
				current_text = ""
				draw_screen()
			}
		} else if ev.MatchesPressOrRepeat("delete") {
			ev.Handled = true
			// Clear any typed hint input first
			current_input = ""
			current_text = ""
			// Close currently selected tab (the one highlighted with arrow keys)
			idx := get_selected_index()
			if idx >= 0 {
				if m := index_map[idx]; m != nil {
					// Mark this as a close action by adding a flag to groupdict
					m_copy := *m
					if m_copy.Groupdict == nil {
						m_copy.Groupdict = make(map[string]any)
					}
					m_copy.Groupdict["close_action"] = true
					chosen = append(chosen, &m_copy)
					lp.Quit(0)
				}
			}
		} else if ev.MatchesPressOrRepeat("enter") || ev.MatchesPressOrRepeat("space") {
			ev.Handled = true
			if current_input != "" {
				// User typed a hint, use that
				idx := decode_hint(current_input, alphabet)
				if m := index_map[idx]; m != nil {
					chosen = append(chosen, m)
					ignore_mark_indices.Add(idx)
					if o.Multiple {
						reset()
						draw_screen()
					} else {
						lp.Quit(0)
					}
				} else {
					current_input = ""
					current_text = ""
					draw_screen()
				}
			} else {
				// No hint typed, use keyboard selection
				idx := get_selected_index()
				if idx >= 0 {
					if m := index_map[idx]; m != nil {
						chosen = append(chosen, m)
						ignore_mark_indices.Add(idx)
						if o.Multiple {
							reset()
							draw_screen()
						} else {
							lp.Quit(0)
						}
					}
				}
			}
		} else if ev.MatchesPressOrRepeat("esc") {
			if o.Multiple {
				lp.Quit(0)
			} else {
				lp.Quit(1)
			}
		}
		return nil
	}

	err = lp.Run()
	if err != nil {
		return 1, err
	}
	ds := lp.DeathSignalName()
	if ds != "" {
		fmt.Println("Killed by signal: ", ds)
		lp.KillIfSignalled()
		return 1, nil
	}
	if lp.ExitCode() != 0 {
		return lp.ExitCode(), nil
	}
	result.Match = make([]string, len(chosen))
	result.Groupdicts = make([]map[string]any, len(chosen))
	for i, m := range chosen {
		result.Match[i] = m.Text + match_suffix
		result.Groupdicts[i] = m.Groupdict
	}
	fmt.Println(output(result))
	return
}

func EntryPoint(parent *cli.Command) {
	create_cmd(parent, main)
}
