#!/usr/bin/env python3
"""Poplar shell composition v4: the four-lens polish review folded.
Changes over v3: symmetric divider clearance (2 cells each side);
rail labels on the chrome edge; cursor-row ink promoted one step
with its ground; rail capped at fg (no unread-white or bold in
chrome); new sub-contrast `border` role carrying the pane divider,
header rule, and modal frame; spinner and sync copy muted and
humanized ("Syncing 12%"); context line count-based and tiered;
reader card subject-first with a To row, symmetric 2/2 margins and
padded interior; quote bar re-glyphed `│` and italicized to match
the snippet's meaning; thread count fixed-width right-aligned;
dates inset 2 with an 8-cell field; footer hint atom uniform (key
plain fg, label fgMuted) with a 6-cell pin gap; modal default
answer as a grounded pill; deeper light grounds so the card edge
survives in light. Open conflicts flagged to Geoff: toast
countdown form (UX-9 wants it visible), attachment glyph.
Throwaway review aid; theme package and gallery are the real
mechanism."""

SLATE = {
    "bg": "16181D", "bgPanel": "262B36", "selectedBg": "2A3441",
    "codeBg": "1D2026", "border": "3A414D",
    "fg": "D4D8DF", "fgMuted": "969DA9", "fgSubtle": "7A8290",
    "accent": "85B3D1", "unread": "ECEEF2",
    "error": "DF8484", "warn": "D4B36A", "success": "97BE8C",
    "link": "85B3D1", "quote": "A0A5AE", "flag": "D4B36A",
}
LIGHT = {
    "bg": "E4E7EC", "bgPanel": "FFFFFF", "selectedBg": "C4CDDB",
    "codeBg": "D9DDE4", "border": "B4BCC9",
    "fg": "262B33", "fgMuted": "4C545F", "fgSubtle": "646D79",
    "accent": "285370", "unread": "12161C",
    "error": "90342E", "warn": "6A4E0A", "success": "3A5A32",
    "link": "285370", "quote": "4A525D", "flag": "6A4E0A",
}
TEXT_ROLES = {"fg", "fgMuted", "unread", "error", "warn", "success",
              "link", "quote", "accent"}
GROUNDS = ("bg", "bgPanel", "selectedBg", "codeBg")


def rgb(h):
    return tuple(int(h[i:i + 2], 16) for i in (0, 2, 4))


def lum(h):
    def f(c):
        c /= 255
        return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4
    r, g, b = rgb(h)
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)


def ratio(a, b):
    la, lb = sorted((lum(a), lum(b)), reverse=True)
    return (la + 0.05) / (lb + 0.05)


R = "\x1b[0m"


def paint(pal, text, role="fg", bold=False, ground="bg", under=False,
          italic=False):
    fr, fgc, fb = rgb(pal[role])
    br, bgc, bb = rgb(pal[ground])
    pre = ("\x1b[1m" if bold else "") + ("\x1b[4m" if under else "") \
        + ("\x1b[3m" if italic else "")
    return pre + f"\x1b[38;2;{fr};{fgc};{fb}m\x1b[48;2;{br};{bgc};{bb}m{text}"


class Row:
    def __init__(self, pal, width, ground="bg"):
        self.pal, self.width, self.ground = pal, width, ground
        self.out, self.used = "", 0

    def seg(self, text, role="fg", bold=False, ground=None, under=False,
            italic=False):
        self.out += paint(self.pal, text, role, bold,
                          ground or self.ground, under, italic)
        self.used += len(text)
        return self

    def pad_to(self, col, ground=None):
        if col > self.used:
            self.seg(" " * (col - self.used), ground=ground)
        return self

    def right(self, segs, ground=None, min_gap=4):
        need = sum(len(t) for t, *_ in segs)
        if self.used + min_gap + need > self.width:
            self.pad_to(self.width)
            return self
        self.pad_to(self.width - need, ground=ground)
        for t, role, bold in segs:
            self.seg(t, role, bold, ground=ground)
        return self

    def done(self):
        self.pad_to(self.width)
        return self.out + R


def ell(s, w):
    return s if len(s) <= w else s[: max(0, w - 1)] + "…"


RAIL_W = 22
PANE_X = RAIL_W + 3          # divider at RAIL_W, 2 cells air each side
GUTTER = 2

SIDEBAR = [
    # name, unread count ("" when none), active
    ("Inbox", "14", True),
    ("Drafts", "", False),
    ("Sent", "", False),
    ("Archive", "", False),
    ("Junk", "1", False),
    ("Trash", "", False),
    ("Lists", "98", False),
]

# unread, attr, cursor, thread, sender, subject, snippet, date
MAIL = [
    (True, "", False, "", "Ada Lovelace", "Analytical engine notes",
     "The engine weaves algebraic patterns just as the loom", "09:41"),
    (False, "", True, "", "Charles Babbage", "Re: difference engine budget",
     "The Treasury insists on itemized brasswork before any", "09:12"),
    (True, "⚑", False, "", "Grace Hopper", "COBOL committee minutes",
     "Attached are the minutes from Thursday. Note the vote", "08:55"),
    (False, "⊕", False, "", "Katherine Johnson", "Trajectory review, Friday",
     "Can you check my numbers for the Friday window? The", "Aug 18"),
    (False, "", False, "", "Fastmail", "Your receipt for August",
     "Thanks for your payment. This month's invoice is", "Aug 17"),
    (True, "", False, "▸ 4", "Frank Lee", "Server migration plan",
     "I propose we move the primary on Saturday night after", "Aug 16"),
    (False, "⊕", False, "", "GitHub", "Code review: sync engine PR #42",
     "poplar/sync: 3 comments on watermark handling from", "Aug 14"),
    (False, "", False, "", "Donald Knuth", "TAOCP errata, volume 4",
     "A reader reports an off-by-one in exercise 7.2.1.5-13", "Aug 13"),
]


def hint(r, key, label, gap_after=0):
    r.seg(key, "fg")
    if label:
        r.seg(" " + label, "fgMuted")
    if gap_after:
        r.seg(" " * gap_after)
    return r


def top_band(pal, width, context=True):
    r = Row(pal, width, ground="bgPanel")
    r.seg("  ").seg("1 Mail", "accent", True).seg("  2 3 4", "fgSubtle")
    if context:
        r.pad_to(PANE_X)
        r.seg("Inbox", "fg")
        r.seg(" · ", "fgSubtle")
        r.seg("12,406 messages", "fgMuted")
        r.seg(" · ", "fgSubtle")
        r.seg("14 unread", "fg")
    sync = "⠹ Syncing 12%" if width < 100 else "⠹ Syncing 4,312 of 36,102"
    r.right([(sync + "  ", "fgMuted", False)], ground="bgPanel")
    return r.done()


def rail_seg(r, i, focused=False):
    if i == 0 or i - 1 >= len(SIDEBAR):
        r.pad_to(RAIL_W)
        r.seg("│", "border")
        return
    name, count, active = SIDEBAR[i - 1]
    ground = "selectedBg" if active else "bg"
    role = "fg" if active else "fgMuted"
    r.seg(("▌" if focused else "▏") if active else " ", "accent",
          ground=ground)
    r.seg(" " + name, role, ground=ground)
    r.pad_to(RAIL_W - len(count) - 2, ground=ground)
    r.seg(count, role, ground=ground)
    r.pad_to(RAIL_W, ground=ground)
    r.seg("│", "border")


def list_seg(r, i, listw, focused=True):
    if i == 0 or i - 1 >= len(MAIL):
        r.pad_to(r.used + listw)
        return
    unread, attr, cursor, thread, sender, subject, snippet, date = \
        MAIL[i - 1]
    ground = "selectedBg" if cursor else "bg"
    # ink promotion on the cursor row: each role steps up one tier
    mut = "fg" if cursor else "fgMuted"
    sub = "fgMuted" if cursor else "fgSubtle"
    start = r.used
    bar = ("▌" if focused else "▏") if cursor else " "
    r.seg(bar, "accent", ground=ground)
    r.seg(" ", ground=ground)
    r.seg("●" if unread else " ", "unread", ground=ground)
    r.seg(" ", ground=ground)
    r.seg((attr or " "), "flag" if attr == "⚑" else "fgMuted",
          ground=ground)
    r.seg("  ", ground=ground)
    r.seg(ell(sender, 17).ljust(17), "unread" if unread else mut,
          unread, ground=ground)
    r.seg("  ", ground=ground)
    r.seg((thread or "").rjust(4), mut, ground=ground)
    r.seg(" ", ground=ground)
    room = listw - (r.used - start) - 8 - 3
    subj = ell(subject, min(len(subject), room))
    r.seg(subj, "fg" if (unread or cursor) else "fgMuted", unread,
          ground=ground)
    room -= len(subj)
    if room > 6:
        r.seg(" · ", sub, ground=ground)
        r.seg(ell(snippet, room - 3), sub, ground=ground, italic=True)
    r.pad_to(start + listw - 8, ground=ground)
    r.seg((date + "  ").rjust(8), mut, ground=ground)


READER = [
    ("subj", "", "Re: difference engine budget"),
    ("pad", "", ""),
    ("h", "From", "Charles Babbage <charles@difference.example>"),
    ("h", "To", "geoff@907.life"),
    ("h", "Date", "Tue 19 Aug 2026, 09:12"),
    ("rule", "", ""),
    ("pad", "", ""),
    ("body", "", "The Treasury insists on itemized brasswork"),
    ("body", "", "before any further disbursement. I enclose"),
    ("body", "", "the schedule of parts."),
    ("pad", "", ""),
    ("quote", "", "your estimate of the mill assembly"),
    ("pad", "", ""),
    ("link", "", "https://example.org/engine-budget"),
    ("pad", "", ""),
    ("code", "", "mill:   4,102 gears"),
    ("code", "", "store:  1,000 columns of 40 wheels"),
    ("pad", "", ""),
    ("attach", "", "⊕ parts-schedule.pdf · 96 KB"),
]


def reader_seg(r, i, readw, pct_row=False, blank=False):
    start = r.used
    if blank or (not pct_row and i >= len(READER)):
        r.pad_to(start + readw, ground="bgPanel")
        return
    if pct_row:
        r.pad_to(start + readw - 5, ground="bgPanel")
        r.seg("34%", "fgSubtle", ground="bgPanel")
        r.pad_to(start + readw, ground="bgPanel")
        return
    kind, label, text = READER[i]
    if kind == "subj":
        r.seg("  " + text, "fg", True, ground="bgPanel")
    elif kind == "h":
        r.seg("  " + label.ljust(7), "fgMuted", ground="bgPanel")
        r.seg(text, "fg", ground="bgPanel")
    elif kind == "rule":
        r.seg("  ", ground="bgPanel")
        r.seg("─" * (readw - 4), "border", ground="bgPanel")
    elif kind == "quote":
        r.seg("  ", ground="bgPanel")
        r.seg("│", "quote", ground="bgPanel")
        r.seg(" " + text, "quote", ground="bgPanel", italic=True)
    elif kind == "link":
        r.seg("  ", ground="bgPanel")
        r.seg(text, "link", ground="bgPanel", under=True)
    elif kind == "code":
        r.seg("  ", ground="bgPanel")
        r.seg("  " + text.ljust(readw - (r.used - start) - 4), "fg",
              ground="codeBg")
    elif kind == "attach":
        r.seg("  " + text, "fgMuted", ground="bgPanel")
    elif kind == "body":
        r.seg("  " + text, "fg", ground="bgPanel")
    r.pad_to(start + readw, ground="bgPanel")


FOOTER_PRIORITY = [
    ("Enter", "open"), ("a", "archive"), ("d", "delete"),
    ("r", "reply"), ("m", "compose"), ("/", "search"),
    ("u", "undo"), ("g", "go to"), ("*", "flag"), ("s", "move"),
    ("Tab", "unread"), ("j/k", "select"),
]
HELP_HINT = ("?", "help")
PIN_GAP = 6


def footer_band(pal, width):
    gap = 3
    def w(h):
        return len(h[0]) + 1 + len(h[1])
    budget = width - 4 - w(HELP_HINT) - PIN_GAP
    shown, used = [], 0
    for h in FOOTER_PRIORITY:
        need = w(h) + (gap if shown else 0)
        if used + need > budget:
            break
        shown.append(h)
        used += need
    r = Row(pal, width, ground="bgPanel").seg("  ")
    for i, (k, d) in enumerate(shown):
        hint(r, k, d, gap if i < len(shown) - 1 else 0)
    r.right([(HELP_HINT[0], "fg", False),
             (" " + HELP_HINT[1] + "  ", "fgMuted", False)],
            ground="bgPanel", min_gap=PIN_GAP)
    return [r.done()]


def frame_standard(pal, width=120):
    listw = width - PANE_X
    rows = [top_band(pal, width)]
    for i in range(12):
        r = Row(pal, width)
        rail_seg(r, i)
        r.pad_to(PANE_X)
        list_seg(r, i, listw)
        rows.append(r.done())
    rows.extend(footer_band(pal, width))
    return rows


def frame_wide(pal, width=150):
    listw = 62
    readw = width - PANE_X - listw - GUTTER - 2
    rows = [top_band(pal, width)]
    last = 23
    for i in range(last + 1):
        r = Row(pal, width)
        rail_seg(r, i)
        r.pad_to(PANE_X)
        list_seg(r, i, listw, focused=True)
        r.pad_to(PANE_X + listw + GUTTER)
        if i in (0, last):
            r.pad_to(r.used + readw)             # base card margins
        elif i in (1, last - 1):
            reader_seg(r, 0, readw, blank=True)  # panel padding rows
        elif i == last - 2:
            reader_seg(r, 0, readw, pct_row=True)
        else:
            reader_seg(r, i - 2, readw)
        rows.append(r.done())
    rows.extend(footer_band(pal, width))
    return rows


def frame_spartan(pal, width=80):
    rows = [top_band(pal, width, context=False)]
    for i in range(9):
        r = Row(pal, width)
        list_seg(r, i, width)
        rows.append(r.done())
    rows.extend(footer_band(pal, width))
    return rows


def banner_strip(pal, width=120):
    toast = Row(pal, width, ground="bgPanel")
    toast.seg("  ").seg("1 Mail", "accent", True).seg("  2 3 4", "fgSubtle")
    toast.pad_to(PANE_X)
    toast.seg("Inbox", "fg").seg(" · ", "fgSubtle")
    toast.seg("12,406 messages", "fgMuted")
    toast.right([("3 messages archived", "fg", False),
                 ("   ", "fg", False), ("u", "fg", False),
                 (" undo", "fgMuted", False), ("  9s  ", "fgSubtle", False)],
                ground="bgPanel")
    r = Row(pal, width, ground="bgPanel")
    r.seg("  ").seg("▲", "warn")
    r.seg(" No keyring found, so your token is stored in a plain file.",
          "fg")
    r.right([("Enter", "fg", False), (" details", "fgMuted", False),
             ("   ", "fg", False), ("Esc", "fg", False),
             (" dismiss", "fgMuted", False), ("  ✕  ", "fgMuted", False)],
            ground="bgPanel")
    return [toast.done(), r.done()]


def modal(pal, width=120):
    box = 46
    pad = (width - box) // 2
    def edge(l, m, r_):
        return (Row(pal, width).pad_to(pad)
                .seg(l + m * (box - 2) + r_, "border",
                     ground="bgPanel").done())
    def inner(segs, centered=False):
        r = Row(pal, width).pad_to(pad)
        r.seg("│", "border", ground="bgPanel")
        used0 = r.used
        if centered:
            w = sum(len(t) for t, *_ in segs)
            r.pad_to(used0 + (box - 2 - w) // 2, ground="bgPanel")
        for seg_ in segs:
            t, role, bold = seg_[0], seg_[1], seg_[2]
            g = seg_[3] if len(seg_) > 3 else "bgPanel"
            r.seg(t, role, bold, ground=g)
        r.pad_to(pad + box - 1, ground="bgPanel")
        r.seg("│", "border", ground="bgPanel")
        return r.done()
    return [edge("╭", "─", "╮"), inner([]),
            inner([("  Quit with 2 unsent messages?", "fg", True)]),
            inner([("  They'll send the next time you open poplar.",
                    "fgMuted", False)]),
            inner([]),
            inner([("y", "fg", False), (" quit", "fgMuted", False),
                   ("   ", "fg", False),
                   ("  n", "fg", False, "selectedBg"),
                   (" stay  ", "fgMuted", False, "selectedBg")],
                  centered=True),
            inner([]), edge("╰", "─", "╯")]


def swatches(pal, name):
    print(f"\n  {name}  (base #{pal['bg']} · panel #{pal['bgPanel']} ·"
          f" selection #{pal['selectedBg']} · code #{pal['codeBg']} ·"
          f" border #{pal['border']}, structural, exempt)\n")
    for role in ("fg", "fgMuted", "fgSubtle", "accent", "unread",
                 "error", "warn", "success", "link", "quote", "flag"):
        h = pal[role]
        need = 4.5 if role in TEXT_ROLES else 3.0
        cells = "".join(
            paint(pal, "  Aa  ", role, role == "unread", g) + R
            for g in GROUNDS)
        worst = min(ratio(h, pal[g]) for g in GROUNDS)
        print(f"  {role:9s} #{h}  {cells}  worst {worst:5.2f}"
              f" (need {need})")


def show(title, rows):
    print(f"\n  {title}\n")
    for r in rows:
        print("  " + r)


print("\n  poplar · composition v4 · four-lens polish review folded")
show("Standard rung 120 · dark", frame_standard(SLATE))
show("Wide rung 150 · dark (subject-first reader, promoted cursor row)",
     frame_wide(SLATE))
show("Toast and banner placement · 120 · dark", banner_strip(SLATE))
show("Modal confirm · dark (border role, default answer grounded)",
     modal(SLATE))
show("Spartan 80 · single pane", frame_spartan(SLATE))
show("Standard rung 120 · light (deepened grounds)",
     frame_standard(LIGHT))
swatches(SLATE, "SLATE dark")
swatches(LIGHT, "SLATE light v4")
print("\n  Flagged for Geoff: toast countdown form (UX-9 mandates a"
      " visible countdown; shown as dim '9s'),\n  and the attachment"
      " glyph (⊕ vs a letter vs a count).")
print("\n  q + Enter (or Ctrl-C) to close")
try:
    input()
except (KeyboardInterrupt, EOFError):
    pass
