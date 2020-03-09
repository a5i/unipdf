package creator

import (
	"bytes"
	"log"
	"strings"

	"github.com/unidoc/unipdf/v3/model"
	"github.com/vanng822/css"
	"golang.org/x/net/html"
)

type HtmlParagraph struct {
	elements         []Drawable
	RegularStyle     TextStyle
	BoldFont         *model.PdfFont
	ItalicFont       *model.PdfFont
	BoldItalicFont   *model.PdfFont
	currentParagraph *StyledParagraph
	styleStack       []htmlTextStyle
	tableStack       []*htmlTable
}

type htmlTableCell struct {
	paragraph *StyledParagraph
}

type htmlTableRow struct {
	cells []*htmlTableCell
}

type htmlTable struct {
	table       *Table
	maxColIndex int
	rows        []*htmlTableRow
}

// GeneratePageBlocks generates the page blocks.  Multiple blocks are generated if the contents wrap
// over multiple pages. Implements the Drawable interface.
func (t *htmlTable) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	if t.table == nil {
		t.table = newTable(t.maxColIndex)
		for _, row := range t.rows {
			for _, cell := range row.cells {
				c := t.table.NewCell()
				c.SetContent(cell.paragraph)
			}
		}
	}
	return t.table.GeneratePageBlocks(ctx)
}

type htmlTextStyle struct {
	TextStyle
	Bold          bool
	Italic        bool
	TextAlignment TextAlignment
}

// NewHtmlParagraph creates a new html paragraph.
// Default attributes:
// Font: Helvetica,
// Font size: 10
// Encoding: WinAnsiEncoding
// Wrap: enabled
// Text color: black
func (c *Creator) NewHtmlParagraph() *HtmlParagraph {
	return newHtmlParagraph(c.NewTextStyle())
}

func newHtmlParagraph(baseStyle TextStyle) *HtmlParagraph {
	hp := HtmlParagraph{
		RegularStyle: baseStyle,
	}
	return &hp
}

// Append adds html to paragraph.
func (h *HtmlParagraph) Append(htmlCode string) error {
	doc, err := html.Parse(bytes.NewBufferString(htmlCode))
	if err != nil {
		return err
	}
	return h.processNode(doc)
}

func (h *HtmlParagraph) currentStyle() htmlTextStyle {
	if len(h.styleStack) == 0 {
		return htmlTextStyle{
			TextStyle: h.RegularStyle,
			Bold:      false,
			Italic:    false,
		}
	}
	return h.styleStack[len(h.styleStack)-1]
}

func (h *HtmlParagraph) pushStyle(style htmlTextStyle) {
	h.styleStack = append(h.styleStack, style)
}

func (h *HtmlParagraph) popStyle() htmlTextStyle {
	s := h.currentStyle()
	if len(h.styleStack) < 1 {
		return s
	}
	h.styleStack = h.styleStack[:len(h.styleStack)-1]
	return s
}

func (h *HtmlParagraph) createAndPushTable() *htmlTable {
	t := &htmlTable{}
	h.pushTable(t)
	return t
}

func (h *HtmlParagraph) currentTable() *htmlTable {
	if len(h.tableStack) == 0 {
		return nil
	}
	return h.tableStack[len(h.tableStack)-1]
}

func (h *HtmlParagraph) pushTable(table *htmlTable) {
	h.tableStack = append(h.tableStack, table)
}

func (h *HtmlParagraph) popTable() *htmlTable {
	t := h.currentTable()
	if len(h.tableStack) < 1 {
		return t
	}
	h.tableStack = h.tableStack[:len(h.tableStack)-1]
	return t
}

func (h *HtmlParagraph) addBold() htmlTextStyle {
	s := h.currentStyle()
	s.Bold = true
	if h.BoldFont != nil {
		s.Font = h.BoldFont
	}
	if s.Italic && h.BoldItalicFont != nil {
		s.Font = h.BoldItalicFont
	}
	return s
}

func (h *HtmlParagraph) addItalic() htmlTextStyle {
	s := h.currentStyle()
	s.Italic = true
	if h.ItalicFont != nil {
		s.Font = h.ItalicFont
	}
	if s.Bold && h.BoldItalicFont != nil {
		s.Font = h.BoldItalicFont
	}
	return s
}

var stdHtmlColors = map[string]Color{
	"blue":   ColorBlue,
	"black":  ColorBlack,
	"green":  ColorGreen,
	"red":    ColorRed,
	"white":  ColorWhite,
	"yellow": ColorYellow,
}

func getRGBColorFromHtml(color string) Color {
	if c, ok := stdHtmlColors[color]; ok {
		return c
	}
	return ColorRGBFromHex(color)
}

func (h *HtmlParagraph) addEmbeddedCSS(tag string, csstext string) htmlTextStyle {
	style := h.currentStyle()
	p := h.getCurrentParagraph()
	ss := css.ParseBlock(csstext)
	for _, s := range ss {
		switch {
		case tag == "p" && s.Property == "text-align":
			switch s.Value {
			case "center":
				p.alignment = TextAlignmentCenter
			case "left":
				p.alignment = TextAlignmentLeft
			case "right":
				p.alignment = TextAlignmentRight
			case "justify":
				p.alignment = TextAlignmentJustify
			}
		case s.Property == "color":
			style.Color = getRGBColorFromHtml(s.Value)
		}
	}
	return style
}

func (h *HtmlParagraph) getCurrentParagraph() *StyledParagraph {
	if h.currentParagraph == nil {
		h.currentParagraph = newStyledParagraph(h.RegularStyle)
		h.elements = append(h.elements, h.currentParagraph)
	}
	return h.currentParagraph
}

var ignoreReplacer = strings.NewReplacer("\r", "", "\n", "", "\t", " ")

func (h *HtmlParagraph) processNode(node *html.Node) error {
	switch node.Type {
	case html.TextNode:
		p := h.getCurrentParagraph()
		text := ignoreReplacer.Replace(node.Data)
		p.Append(text).Style = h.currentStyle().TextStyle
		return nil
	case html.ElementNode:
		switch node.Data {
		case "style":
			log.Println(node)
			return nil
		case "script":
			return nil
		case "table":
			t := h.createAndPushTable()
			h.elements = append(h.elements, t)
			defer h.popTable()
		case "tr":
			if t := h.currentTable(); t != nil {
				t.rows = append(t.rows, &htmlTableRow{})
			}
		case "td", "th":
			if t := h.currentTable(); t != nil && len(t.rows) > 0 {
				row := t.rows[len(t.rows)-1]
				h.currentParagraph = newStyledParagraph(h.currentStyle().TextStyle)
				cell := htmlTableCell{paragraph: h.currentParagraph}
				row.cells = append(row.cells, &cell)
				if l := len(row.cells); l > t.maxColIndex {
					t.maxColIndex = l
				}
			}
			if node.Data == "th" {
				h.pushStyle(h.addBold())
				defer h.popStyle()
			}
		case "p":
			h.currentParagraph = newStyledParagraph(h.RegularStyle)
			h.elements = append(h.elements, h.currentParagraph)
		case "div", "br":
			p := h.getCurrentParagraph()
			p.Append("\n")
		case "b":
			h.pushStyle(h.addBold())
			defer h.popStyle()
		case "i":
			h.pushStyle(h.addItalic())
			defer h.popStyle()
		}
		for _, attr := range node.Attr {
			switch attr.Key {
			case "style":
				h.pushStyle(h.addEmbeddedCSS(node.Data, attr.Val))
				defer h.popStyle()
			case "align":
				if node.Data == "td" || node.Data == "th" {
					switch attr.Val {
					case "center":
						h.currentParagraph.alignment = TextAlignmentCenter
					case "left":
						h.currentParagraph.alignment = TextAlignmentLeft
					case "right":
						h.currentParagraph.alignment = TextAlignmentRight
					case "justify":
						h.currentParagraph.alignment = TextAlignmentJustify
					}
				}
			}
		}
	}

	for next := node.FirstChild; next != nil; next = next.NextSibling {
		if err := h.processNode(next); err != nil {
			return err
		}
	}
	return nil
}

// GeneratePageBlocks generates the page blocks.  Multiple blocks are generated if the contents wrap
// over multiple pages. Implements the Drawable interface.
func (h *HtmlParagraph) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	var blocks []*Block
	origCtx := ctx
	for _, e := range h.elements {
		var newBlocks []*Block
		var err error
		newBlocks, ctx, err = e.GeneratePageBlocks(ctx)
		if err != nil {
			return nil, ctx, err
		}
		if len(newBlocks) < 1 {
			continue
		}
		if len(blocks) == 0 {
			blocks = newBlocks[0:1]
		} else {
			blocks[len(blocks)-1].mergeBlocks(newBlocks[0])
		}

		blocks = append(blocks, newBlocks[1:]...)
	}
	return blocks, origCtx, nil
}
