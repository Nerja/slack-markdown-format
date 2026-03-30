package slackmarkdownformat

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

func TestConvertToBlocks(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		expected [][]slack.Block
	}{
		{
			name:     "empty string",
			markdown: "",
			expected: [][]slack.Block{},
		},
		{
			name:     "whitespace only",
			markdown: "   \n\n   ",
			expected: [][]slack.Block{},
		},
		{
			name:     "simple text",
			markdown: "Hello, world!",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "Hello, world!")},
			},
		},
		{
			name:     "single column table",
			markdown: "| A |\n|---|\n| B |",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "| A |\n|---|\n| B |")},
			},
		},
		{
			name:     "two columns table",
			markdown: "| A | B |\n|---|---|\n| A1 | B1 |\n| A2 | B2 |",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "| A | B |\n|---|---|\n| A1 | B1 |\n| A2 | B2 |")},
			},
		},
		{
			name: "text before and after table",
			markdown: lines(
				"Here is some intro text.",
				"",
				"| Name | Value |",
				"|------|-------|",
				"| Foo  | 123   |",
				"",
				"And here is some closing text.",
			),
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", lines(
					"Here is some intro text.",
					"",
					"| Name | Value |",
					"|------|-------|",
					"| Foo  | 123   |",
					"",
					"And here is some closing text.",
				))},
			},
		},
		{
			name: "two tables split into two messages",
			markdown: lines(
				"First table:",
				"",
				"| A | B |",
				"|---|---|",
				"| 1 | 2 |",
				"",
				"Second table:",
				"",
				"| X | Y |",
				"|---|---|",
				"| 3 | 4 |",
			),
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", lines(
					"First table:",
					"",
					"| A | B |",
					"|---|---|",
					"| 1 | 2 |",
					"",
					"Second table:",
				))},
				{slack.NewMarkdownBlock("", "| X | Y |\n|---|---|\n| 3 | 4 |")},
			},
		},
		{
			name:     "code block with yaml",
			markdown: "Here is some config:\n\n```yaml\nname: my-app\nversion: 1.0.0\n```",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "Here is some config:\n\n```yaml\nname: my-app\nversion: 1.0.0\n```")},
			},
		},
		{
			name:     "code block without language",
			markdown: "```\nplain code\n```",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "```\nplain code\n```")},
			},
		},
		{
			name:     "consecutive tables split into separate messages",
			markdown: "| A |\n|---|\n| 1 |\n\n| B |\n|---|\n| 2 |",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "| A |\n|---|\n| 1 |")},
				{slack.NewMarkdownBlock("", "| B |\n|---|\n| 2 |")},
			},
		},
		{
			name:     "multiple paragraphs",
			markdown: "First paragraph.\n\nSecond paragraph.",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "First paragraph.\n\nSecond paragraph.")},
			},
		},
		{
			name: "all supported elements in single message",
			markdown: lines(
				"# Main Heading",
				"",
				"This is an introductory paragraph.",
				"",
				"## Subheading",
				"",
				"Here is a list of items:",
				"",
				"- First item",
				"- Second item",
				"- Third item",
				"",
				"1. First",
				"2. Second",
				"3. Third",
				"",
				"> This is a blockquote with some",
				"> important information.",
				"",
				"### Another Heading",
				"",
				"| Name | Status |",
				"|------|--------|",
				"| Foo  | Active |",
				"| Bar  | Pending |",
				"",
				"And some final text.",
				"",
				"```yaml",
				"config:",
				"  enabled: true",
				"```",
			),
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", lines(
					"# Main Heading",
					"",
					"This is an introductory paragraph.",
					"",
					"## Subheading",
					"",
					"Here is a list of items:",
					"",
					"- First item",
					"- Second item",
					"- Third item",
					"",
					"1. First",
					"2. Second",
					"3. Third",
					"",
					"> This is a blockquote with some",
					"> important information.",
					"",
					"### Another Heading",
					"",
					"| Name | Status |",
					"|------|--------|",
					"| Foo  | Active |",
					"| Bar  | Pending |",
					"",
					"And some final text.",
					"",
					"```yaml",
					"config:",
					"  enabled: true",
					"```",
				))},
			},
		},
		{
			name:     "thematic break preserved",
			markdown: "Before\n\n---\n\nAfter",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "Before\n\n---\n\nAfter")},
			},
		},
		{
			name:     "markdown limit split into multiple messages",
			markdown: strings.Repeat("a", 7000) + "\n\n" + strings.Repeat("b", 7000),
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", strings.Repeat("a", 7000))},
				{slack.NewMarkdownBlock("", strings.Repeat("b", 7000))},
			},
		},
		{
			name:     "three paragraphs where first two fit but third overflows",
			markdown: strings.Repeat("a", 5000) + "\n\n" + strings.Repeat("b", 5000) + "\n\n" + strings.Repeat("c", 5000),
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", strings.Repeat("a", 5000)+"\n\n"+strings.Repeat("b", 5000))},
				{slack.NewMarkdownBlock("", strings.Repeat("c", 5000))},
			},
		},
		{
			name:     "single paragraph exceeding limit is hard-split",
			markdown: strings.Repeat("x", 13000),
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", strings.Repeat("x", 12000))},
				{slack.NewMarkdownBlock("", strings.Repeat("x", 1000))},
			},
		},
		{
			name:     "text before table stays in same message",
			markdown: "# Title\n\nSome intro.\n\n| A | B |\n|---|---|\n| 1 | 2 |",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "# Title\n\nSome intro.\n\n| A | B |\n|---|---|\n| 1 | 2 |")},
			},
		},
		{
			name:     "text between tables groups with preceding table",
			markdown: "Intro\n\n| A |\n|---|\n| 1 |\n\nMiddle text\n\n| B |\n|---|\n| 2 |\n\nOutro",
			expected: [][]slack.Block{
				{slack.NewMarkdownBlock("", "Intro\n\n| A |\n|---|\n| 1 |\n\nMiddle text")},
				{slack.NewMarkdownBlock("", "| B |\n|---|\n| 2 |\n\nOutro")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToBlocks(tt.markdown)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertToBlocks():\ngot:      %s\nexpected: %s",
					formatMessages(got), formatMessages(tt.expected))
			}
		})
	}
}

func TestSplitByParagraphs(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxChars int
		expected []string
	}{
		{
			name:     "fits in one chunk",
			text:     "hello world",
			maxChars: 100,
			expected: []string{"hello world"},
		},
		{
			name:     "split at paragraph boundary",
			text:     "aaa\n\nbbb",
			maxChars: 5,
			expected: []string{"aaa", "bbb"},
		},
		{
			name:     "hard split single paragraph",
			text:     "abcdef",
			maxChars: 3,
			expected: []string{"abc", "def"},
		},
		{
			name:     "paragraphs combined when they fit",
			text:     "a\n\nb\n\nc",
			maxChars: 100,
			expected: []string{"a\n\nb\n\nc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitByParagraphs(tt.text, tt.maxChars)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("splitByParagraphs(%q, %d):\ngot:      %q\nexpected: %q",
					tt.text, tt.maxChars, got, tt.expected)
			}
		})
	}
}

// lines joins its arguments with newlines, making multi-line test strings
// readable without long inline concatenations.
func lines(s ...string) string {
	return strings.Join(s, "\n")
}

func formatMessages(msgs [][]slack.Block) string {
	var b strings.Builder
	for i, msg := range msgs {
		for j, blk := range msg {
			if i > 0 || j > 0 {
				b.WriteString("\n          ")
			}
			if mb, ok := blk.(*slack.MarkdownBlock); ok {
				fmt.Fprintf(&b, "msg[%d][%d]: markdown=%q", i, j, mb.Text)
			} else {
				fmt.Fprintf(&b, "msg[%d][%d]: %T", i, j, blk)
			}
		}
	}
	return b.String()
}
