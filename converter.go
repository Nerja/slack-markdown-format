// Package slackmarkdownformat converts standard Markdown into Slack
// markdown blocks, splitting across messages where necessary to respect
// Slack's one-table-per-message and 12,000-character limits.
package slackmarkdownformat

import (
	"strings"
	"unicode/utf8"

	"github.com/slack-go/slack"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

const (
	maxMarkdownCharsPerMessage = 12000
	paragraphSeparator         = "\n\n"
)

// ConvertToBlocks converts a markdown string to Slack markdown blocks.
// Returns a slice of messages, where each message is a slice of blocks.
// Messages are split to stay within Slack's limits: one table per message
// and 12,000 cumulative markdown characters per message. Slack renders the
// markdown server-side, so the raw markdown is passed through as-is.
func ConvertToBlocks(markdown string) [][]slack.Block {
	source := []byte(markdown)
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(source))

	segments := splitAtTables(doc, source)
	if len(segments) == 0 {
		return [][]slack.Block{}
	}

	var b messageBuilder
	for _, seg := range segments {
		b.addSegment(seg)
	}
	b.flush()
	return b.messages
}

type messageBuilder struct {
	messages [][]slack.Block
	current  string
	hasTable bool
}

func (b *messageBuilder) addSegment(seg segment) {
	if seg.isTable && b.hasTable {
		b.flush()
	}

	for _, part := range splitByParagraphs(seg.text, maxMarkdownCharsPerMessage) {
		combined := joinMarkdown(b.current, part)
		if runeLen(combined) > maxMarkdownCharsPerMessage {
			b.flush()
			b.current = part
		} else {
			b.current = combined
		}
	}

	if seg.isTable {
		b.hasTable = true
	}
}

func (b *messageBuilder) flush() {
	if b.current == "" {
		return
	}
	b.messages = append(b.messages, []slack.Block{
		slack.NewMarkdownBlock("", b.current),
	})
	b.current = ""
	b.hasTable = false
}

// segment represents a contiguous range of raw markdown source.
// Tables are isolated into their own segments so the caller can enforce
// Slack's one-table-per-message limit.
type segment struct {
	text    string
	isTable bool
}

// splitAtTables divides the source into segments, splitting only at table
// boundaries. Consecutive non-table nodes are merged into a single segment.
func splitAtTables(doc ast.Node, source []byte) []segment {
	var segments []segment
	var runStart, runEnd int
	runActive := false

	appendSegment := func(start, end int, isTable bool) {
		if start >= end {
			return
		}
		raw := strings.TrimRight(string(source[start:end]), "\n")
		if raw != "" {
			segments = append(segments, segment{text: raw, isTable: isTable})
		}
	}

	flushRun := func() {
		if !runActive {
			return
		}
		appendSegment(runStart, runEnd, false)
		runActive = false
	}

	starts := nodeStartBytes(doc, source)

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		start, end := nodeByteRange(n, source, starts)
		if start >= end {
			continue
		}

		if n.Kind() == east.KindTable {
			flushRun()
			appendSegment(start, end, true)
		} else {
			if !runActive {
				runStart = start
				runActive = true
			}
			runEnd = end
		}
	}
	flushRun()

	return segments
}

// nodeStartBytes pre-computes the start byte offset for every top-level
// child node in a single pass, avoiding redundant AST walks.
func nodeStartBytes(doc ast.Node, source []byte) map[ast.Node]int {
	starts := make(map[ast.Node]int)
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		starts[n] = nodeStartByte(n, source)
	}
	return starts
}

// nodeByteRange returns the byte range [start, end) in source that covers
// the entire raw text for a top-level AST node. It uses the pre-computed
// start of the next sibling as the end boundary.
func nodeByteRange(n ast.Node, source []byte, starts map[ast.Node]int) (int, int) {
	start, ok := starts[n]
	if !ok || start < 0 {
		return 0, 0
	}

	end := len(source)
	if next := n.NextSibling(); next != nil {
		if nextStart, ok := starts[next]; ok && nextStart >= 0 {
			end = nextStart
		}
	}

	return start, end
}

// nodeStartByte finds the first byte offset of a node by examining its
// descendants' line segments and inline text segments, then walking
// backwards to the beginning of the line to capture markdown syntax.
func nodeStartByte(n ast.Node, source []byte) int {
	minStart := -1

	if fcb, ok := n.(*ast.FencedCodeBlock); ok && fcb.Info != nil {
		minStart = fcb.Info.Segment.Start
	}

	ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if child.Type() == ast.TypeInline {
			if child.Kind() == ast.KindText {
				s := child.(*ast.Text).Segment.Start
				if minStart < 0 || s < minStart {
					minStart = s
				}
			}
			return ast.WalkContinue, nil
		}

		if lines := child.Lines(); lines.Len() > 0 {
			s := lines.At(0).Start
			if minStart < 0 || s < minStart {
				minStart = s
			}
		}
		return ast.WalkContinue, nil
	})

	if minStart < 0 {
		return -1
	}

	// Walk backwards to the start of the line to capture leading markdown
	// syntax characters (e.g. #, ```, |, -, >).
	for minStart > 0 && source[minStart-1] != '\n' {
		minStart--
	}

	// For fenced code blocks without a language (Info == nil), the content
	// starts on the line after the opening fence. Back up past it.
	if fcb, ok := n.(*ast.FencedCodeBlock); ok && fcb.Info == nil {
		if minStart > 0 && source[minStart-1] == '\n' {
			fence := minStart - 1
			for fence > 0 && source[fence-1] != '\n' {
				fence--
			}
			if strings.HasPrefix(strings.TrimSpace(string(source[fence:minStart-1])), "```") {
				minStart = fence
			}
		}
	}

	return minStart
}

func joinMarkdown(a, b string) string {
	if a == "" {
		return b
	}
	return a + paragraphSeparator + b
}

// splitByParagraphs splits text into chunks that fit within maxChars,
// preferring paragraph boundaries. Falls back to hard rune-level splits
// for single paragraphs that exceed the limit.
func splitByParagraphs(text string, maxChars int) []string {
	if runeLen(text) <= maxChars {
		return []string{text}
	}

	paragraphs := strings.Split(text, paragraphSeparator)
	var chunks []string
	var current string

	for _, p := range paragraphs {
		if runeLen(p) > maxChars {
			if current != "" {
				chunks = append(chunks, current)
				current = ""
			}
			chunks = append(chunks, hardSplitRunes(p, maxChars)...)
			continue
		}

		combined := joinMarkdown(current, p)
		if runeLen(combined) > maxChars {
			chunks = append(chunks, current)
			current = p
		} else {
			current = combined
		}
	}

	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

// hardSplitRunes splits s into chunks of at most maxChars runes each.
func hardSplitRunes(s string, maxChars int) []string {
	runes := []rune(s)
	chunks := make([]string, 0, (len(runes)+maxChars-1)/maxChars)
	for start := 0; start < len(runes); start += maxChars {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}
