package slackmarkdownformat

import (
	"reflect"
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
			name:     "simple text",
			markdown: "Hello, world!",
			expected: [][]slack.Block{
				{
					slack.NewMarkdownBlock("", "Hello, world!"),
				},
			},
		},
		{
			name: "single column table",
			markdown: `| A |
|---|
| B |`,
			expected: [][]slack.Block{
				{
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("A")},
							{newCell("B")},
						},
					},
				},
			},
		},
		{
			name: "two columns table",
			markdown: `| A | B |
|---|---|
| A1 | B1 |
| A2 | B2 |`,
			expected: [][]slack.Block{
				{
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("A"), newCell("B")},
							{newCell("A1"), newCell("B1")},
							{newCell("A2"), newCell("B2")},
						},
					},
				},
			},
		},
		{
			name: "text before and after table",
			markdown: `Here is some intro text.

| Name | Value |
|------|-------|
| Foo  | 123   |

And here is some closing text.`,
			expected: [][]slack.Block{
				{
					slack.NewMarkdownBlock("", "Here is some intro text."),
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("Name"), newCell("Value")},
							{newCell("Foo"), newCell("123")},
						},
					},
					slack.NewMarkdownBlock("", "And here is some closing text."),
				},
			},
		},
		{
			name: "two tables split into two messages",
			markdown: `First table:

| A | B |
|---|---|
| 1 | 2 |

Second table:

| X | Y |
|---|---|
| 3 | 4 |`,
			expected: [][]slack.Block{
				{
					slack.NewMarkdownBlock("", "First table:"),
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("A"), newCell("B")},
							{newCell("1"), newCell("2")},
						},
					},
					slack.NewMarkdownBlock("", "Second table:"),
				},
				{
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("X"), newCell("Y")},
							{newCell("3"), newCell("4")},
						},
					},
				},
			},
		},
		{
			name:     "code block with yaml",
			markdown: "Here is some config:\n\n```yaml\nname: my-app\nversion: 1.0.0\n```",
			expected: [][]slack.Block{
				{
					slack.NewMarkdownBlock("", "Here is some config:"),
					slack.NewMarkdownBlock("", "```yaml\nname: my-app\nversion: 1.0.0\n```"),
				},
			},
		},
		{
			name:     "code block without language",
			markdown: "```\nplain code\n```",
			expected: [][]slack.Block{
				{
					slack.NewMarkdownBlock("", "```\nplain code\n```"),
				},
			},
		},
		{
			name: "consecutive tables split into separate messages",
			markdown: `| A |
|---|
| 1 |

| B |
|---|
| 2 |`,
			expected: [][]slack.Block{
				{
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("A")},
							{newCell("1")},
						},
					},
				},
				{
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("B")},
							{newCell("2")},
						},
					},
				},
			},
		},
		{
			name:     "multiple paragraphs",
			markdown: "First paragraph.\n\nSecond paragraph.",
			expected: [][]slack.Block{
				{
					slack.NewMarkdownBlock("", "First paragraph."),
					slack.NewMarkdownBlock("", "Second paragraph."),
				},
			},
		},
		{
			name: "all supported elements",
			markdown: `# Main Heading

This is an introductory paragraph.

## Subheading

Here is a list of items:

- First item
- Second item
- Third item

1. First
2. Second
3. Third

> This is a blockquote with some
> important information.

### Another Heading

| Name | Status |
|------|--------|
| Foo  | Active |
| Bar  | Pending |

And some final text.

` + "```yaml\nconfig:\n  enabled: true\n```",
			expected: [][]slack.Block{
				{
					slack.NewMarkdownBlock("", "# Main Heading"),
					slack.NewMarkdownBlock("", "This is an introductory paragraph."),
					slack.NewMarkdownBlock("", "## Subheading"),
					slack.NewMarkdownBlock("", "Here is a list of items:"),
					slack.NewMarkdownBlock("", "- First item\n- Second item\n- Third item"),
					slack.NewMarkdownBlock("", "1. First\n2. Second\n3. Third"),
					slack.NewMarkdownBlock("", "This is a blockquote with some\nimportant information."),
					slack.NewMarkdownBlock("", "### Another Heading"),
					&slack.TableBlock{
						Type: slack.MBTTable,
						Rows: [][]*slack.RichTextBlock{
							{newCell("Name"), newCell("Status")},
							{newCell("Foo"), newCell("Active")},
							{newCell("Bar"), newCell("Pending")},
						},
					},
					slack.NewMarkdownBlock("", "And some final text."),
					slack.NewMarkdownBlock("", "```yaml\nconfig:\n  enabled: true\n```"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToBlocks(tt.markdown)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertToBlocks() =\n%+v\nexpected:\n%+v", got, tt.expected)
			}
		})
	}
}

func newCell(text string) *slack.RichTextBlock {
	return slack.NewRichTextBlock("",
		slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement(text, nil),
		),
	)
}
