package slackmarkdownformat

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// ConvertToBlocks converts a markdown string to Slack blocks.
// Returns a slice of messages, where each message is a slice of blocks.
// Messages are split when multiple tables are encountered, since Slack
// only supports one table per message.
func ConvertToBlocks(markdown string) [][]slack.Block {
	source := []byte(markdown)
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	node := md.Parser().Parse(text.NewReader(source))

	messages := [][]slack.Block{}
	var currentMessage []slack.Block
	hasTable := false

	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// Handle tables specially (Slack requires TableBlock)
		if n.Kind() == east.KindTable {
			if hasTable {
				messages = append(messages, currentMessage)
				currentMessage = nil
			}
			currentMessage = append(currentMessage, convertTable(n, source))
			hasTable = true
			return ast.WalkSkipChildren, nil
		}

		// Convert other block elements to MarkdownBlock
		if text := nodeToMarkdown(n, source); text != "" {
			currentMessage = append(currentMessage, slack.NewMarkdownBlock("", text))
			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})

	if len(currentMessage) > 0 {
		messages = append(messages, currentMessage)
	}

	return messages
}

// nodeToMarkdown converts supported node types to markdown text.
// Returns empty string for unsupported or container nodes.
func nodeToMarkdown(n ast.Node, source []byte) string {
	switch n.Kind() {
	case ast.KindHeading:
		h := n.(*ast.Heading)
		return strings.Repeat("#", h.Level) + " " + extractText(n, source)
	case ast.KindParagraph:
		return extractRawLines(n, source)
	case ast.KindFencedCodeBlock:
		cb := n.(*ast.FencedCodeBlock)
		lang := string(cb.Language(source))
		code := extractRawLines(n, source)
		return "```" + lang + "\n" + code + "\n```"
	case ast.KindList:
		return extractList(n.(*ast.List), source)
	}
	return ""
}

// extractRawLines extracts raw source from a node's lines.
func extractRawLines(n ast.Node, source []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write(line.Value(source))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// extractList reconstructs markdown for a list.
func extractList(list *ast.List, source []byte) string {
	var items []string
	num := 1
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		marker := "-"
		if list.IsOrdered() {
			marker = fmt.Sprintf("%d.", num)
			num++
		}
		items = append(items, marker+" "+extractText(item, source))
	}
	return strings.Join(items, "\n")
}

// extractText extracts plain text from a node and its children.
func extractText(n ast.Node, source []byte) string {
	var b strings.Builder
	ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && child.Kind() == ast.KindText {
			b.Write(child.(*ast.Text).Segment.Value(source))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// convertTable converts a goldmark Table node to a slack.TableBlock.
func convertTable(n ast.Node, source []byte) *slack.TableBlock {
	table := slack.NewTableBlock("")
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []*slack.RichTextBlock
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, slack.NewRichTextBlock("",
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement(extractText(cell, source), nil),
				),
			))
		}
		table.AddRow(cells...)
	}
	return table
}
