package main

import (
	"fmt"
	"html"
	"strings"

	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

func renderMattermostMarkdownToConfluenceStorage(markdown string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Linkify,
			extension.Strikethrough,
			extension.Table,
			extension.TaskList,
		),
	)

	source := []byte(markdown)
	doc := md.Parser().Parse(text.NewReader(source))

	var b strings.Builder
	renderConfluenceStorageNode(&b, source, doc)
	return b.String(), nil
}

func renderConfluenceStorageNode(b *strings.Builder, source []byte, node ast.Node) {
	switch n := node.(type) {
	case *ast.Document:
		renderChildren(b, source, n)
	case *ast.Paragraph:
		b.WriteString("<p>")
		renderChildren(b, source, n)
		b.WriteString("</p>")
	case *ast.Heading:
		level := n.Level
		if level < 1 || level > 6 {
			level = 1
		}
		fmt.Fprintf(b, "<h%d>", level)
		renderChildren(b, source, n)
		fmt.Fprintf(b, "</h%d>", level)
	case *ast.Text:
		b.WriteString(html.EscapeString(string(n.Segment.Value(source))))
		if n.HardLineBreak() || n.SoftLineBreak() {
			b.WriteString("<br />")
		}
	case *ast.String:
		b.WriteString(html.EscapeString(string(n.Value)))
	case *ast.Emphasis:
		tag := "em"
		if n.Level == 2 {
			tag = "strong"
		}
		b.WriteString("<" + tag + ">")
		renderChildren(b, source, n)
		b.WriteString("</" + tag + ">")
	case *extast.Strikethrough:
		b.WriteString("<span style=\"text-decoration: line-through;\">")
		renderChildren(b, source, n)
		b.WriteString("</span>")
	case *ast.Link:
		b.WriteString("<a href=\"")
		b.WriteString(html.EscapeString(string(n.Destination)))
		b.WriteString("\">")
		renderChildren(b, source, n)
		b.WriteString("</a>")
	case *ast.AutoLink:
		url := string(n.URL(source))
		b.WriteString("<a href=\"")
		b.WriteString(html.EscapeString(url))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(url))
		b.WriteString("</a>")
	case *ast.CodeSpan:
		b.WriteString("<code>")
		renderChildren(b, source, n)
		b.WriteString("</code>")
	case *ast.Blockquote:
		b.WriteString("<blockquote>")
		renderChildren(b, source, n)
		b.WriteString("</blockquote>")
	case *ast.List:
		tag := "ul"
		if n.IsOrdered() {
			tag = "ol"
		}
		b.WriteString("<" + tag + ">")
		renderChildren(b, source, n)
		b.WriteString("</" + tag + ">")
	case *ast.ListItem:
		b.WriteString("<li>")
		renderChildren(b, source, n)
		b.WriteString("</li>")
	case *ast.FencedCodeBlock:
		renderCodeBlockMacro(b, source, n.Language(source), n.Lines())
	case *ast.CodeBlock:
		renderCodeBlockMacro(b, source, nil, n.Lines())
	case *ast.ThematicBreak:
		b.WriteString("<hr />")
	case *extast.Table:
		b.WriteString("<table><tbody>")
		renderChildren(b, source, n)
		b.WriteString("</tbody></table>")
	case *extast.TableHeader:
		b.WriteString("<tr>")
		renderChildren(b, source, n)
		b.WriteString("</tr>")
	case *extast.TableRow:
		b.WriteString("<tr>")
		renderChildren(b, source, n)
		b.WriteString("</tr>")
	case *extast.TableCell:
		tag := "td"
		if n.Parent() != nil {
			if _, ok := n.Parent().Parent().(*extast.TableHeader); ok {
				tag = "th"
			}
		}
		b.WriteString("<" + tag + ">")
		renderChildren(b, source, n)
		b.WriteString("</" + tag + ">")
	default:
		renderChildren(b, source, node)
	}
}

func renderChildren(b *strings.Builder, source []byte, node ast.Node) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		renderConfluenceStorageNode(b, source, child)
	}
}

func renderCodeBlockMacro(b *strings.Builder, source, language []byte, lines *text.Segments) {
	b.WriteString("<ac:structured-macro ac:name=\"code\">")
	if lang := strings.TrimSpace(string(language)); lang != "" {
		b.WriteString("<ac:parameter ac:name=\"language\">")
		b.WriteString(html.EscapeString(lang))
		b.WriteString("</ac:parameter>")
	}
	b.WriteString("<ac:plain-text-body><![CDATA[")
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		b.Write(segment.Value(source))
	}
	b.WriteString("]]></ac:plain-text-body></ac:structured-macro>")
}
