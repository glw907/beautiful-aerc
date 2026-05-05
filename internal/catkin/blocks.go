package catkin

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// BlockKind identifies the markdown block role of a single line.
type BlockKind int

const (
	BlockBlank BlockKind = iota
	BlockParagraph
	BlockHeading
	BlockQuote
	BlockListItem
	BlockTaskItem
	BlockCodeFence
	BlockCodeIndent
	BlockTable
)

// LineContext describes a single line's block role.
type LineContext struct {
	Kind         BlockKind
	QuoteDepth   int
	ListMarker   string
	HeadingLevel int
	InsideFence  bool
	PrefixWidth  int
	PostPrefix   string
}

var (
	atxHeadingRE  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	taskItemRE    = regexp.MustCompile(`^([-*+])\s+(\[[ xX]\])\s+(.*)$`)
	dashListRE    = regexp.MustCompile(`^([-*+])\s+(.*)$`)
	orderedListRE = regexp.MustCompile(`^(\d+\.)\s+(.*)$`)
	tableRowRE    = regexp.MustCompile(`^\s*\|.*\|\s*$`)
)

// Classify returns a LineContext for each line in lines, in order.
func Classify(lines []string) []LineContext {
	out := make([]LineContext, len(lines))
	insideFence := false

	for i, raw := range lines {
		line := raw
		ctx := LineContext{}

		// Quote prefix walk.
		for {
			s := strings.TrimPrefix(line, ">")
			if s == line {
				break
			}
			ctx.QuoteDepth++
			ctx.PrefixWidth++
			s = strings.TrimPrefix(s, " ")
			if len(s) != len(strings.TrimPrefix(line, ">")) {
				ctx.PrefixWidth++
			}
			line = s
		}

		if insideFence {
			if isFenceMarker(line) {
				ctx.Kind = BlockCodeFence
				ctx.InsideFence = false
				ctx.PostPrefix = line
				insideFence = false
			} else {
				ctx.Kind = BlockCodeFence
				ctx.InsideFence = true
				ctx.PostPrefix = line
			}
			out[i] = ctx
			continue
		}

		if isFenceMarker(line) {
			ctx.Kind = BlockCodeFence
			ctx.PostPrefix = line
			insideFence = true
			out[i] = ctx
			continue
		}

		if strings.TrimSpace(line) == "" {
			ctx.Kind = BlockBlank
			out[i] = ctx
			continue
		}

		if ctx.QuoteDepth == 0 && strings.HasPrefix(line, "    ") {
			ctx.Kind = BlockCodeIndent
			ctx.PrefixWidth = 4
			ctx.PostPrefix = strings.TrimPrefix(line, "    ")
			out[i] = ctx
			continue
		}

		if m := atxHeadingRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockHeading
			ctx.HeadingLevel = len(m[1])
			ctx.PostPrefix = m[2]
			out[i] = ctx
			continue
		}

		if m := taskItemRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockTaskItem
			ctx.ListMarker = m[1] + " " + m[2]
			ctx.PrefixWidth += utf8.RuneCountInString(ctx.ListMarker) + 1
			ctx.PostPrefix = m[3]
			out[i] = ctx
			continue
		}

		if m := dashListRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockListItem
			ctx.ListMarker = m[1]
			ctx.PrefixWidth += utf8.RuneCountInString(m[1]) + 1
			ctx.PostPrefix = m[2]
			out[i] = ctx
			continue
		}

		if m := orderedListRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockListItem
			ctx.ListMarker = m[1]
			ctx.PrefixWidth += utf8.RuneCountInString(m[1]) + 1
			ctx.PostPrefix = m[2]
			out[i] = ctx
			continue
		}

		if tableRowRE.MatchString(line) {
			ctx.Kind = BlockTable
			ctx.PostPrefix = line
			out[i] = ctx
			continue
		}

		if ctx.QuoteDepth > 0 {
			ctx.Kind = BlockQuote
		} else {
			ctx.Kind = BlockParagraph
		}
		ctx.PostPrefix = line
		out[i] = ctx
	}

	return out
}

func isFenceMarker(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}
