package creator

import (
	"bytes"
	"github.com/vanng822/css"
	"log"
	"math"
	"strings"

	"github.com/unidoc/unipdf/v3/model"
	"golang.org/x/net/html"
)

type htmlTextStyle struct {
	TextStyle
	Bold          bool
	Italic        bool
	TextAlignment TextAlignment
}

func (style *htmlTextStyle) addEmbeddedCSS(tag string, csstext string) {
	ss := css.ParseBlock(csstext)
	for _, s := range ss {
		switch {
		case s.Property == "text-align":
			switch s.Value {
			case "center":
				style.TextAlignment = TextAlignmentCenter
			case "left":
				style.TextAlignment = TextAlignmentLeft
			case "right":
				style.TextAlignment = TextAlignmentRight
			case "justify":
				style.TextAlignment = TextAlignmentJustify
			}
		case s.Property == "color":
			style.Color = getRGBColorFromHtml(s.Value)
		}
	}
}

type htmlStyleStack struct {
	RegularStyle   TextStyle
	BoldFont       *model.PdfFont
	ItalicFont     *model.PdfFont
	BoldItalicFont *model.PdfFont
	styleStack     []htmlTextStyle
}

func (s *htmlStyleStack) currentStyle() htmlTextStyle {
	if len(s.styleStack) == 0 {
		return htmlTextStyle{
			TextStyle: s.RegularStyle,
			Bold:      false,
			Italic:    false,
		}
	}
	return s.styleStack[len(s.styleStack)-1]
}

func (s *htmlStyleStack) pushStyle(style htmlTextStyle) {
	s.styleStack = append(s.styleStack, style)
}

func (s *htmlStyleStack) popStyle() htmlTextStyle {
	style := s.currentStyle()
	if len(s.styleStack) < 1 {
		return style
	}
	s.styleStack = s.styleStack[:len(s.styleStack)-1]
	return style
}

func (s *htmlStyleStack) addBold() htmlTextStyle {
	style := s.currentStyle()
	style.Bold = true
	if s.BoldFont != nil {
		style.Font = s.BoldFont
	}
	if style.Italic && s.BoldItalicFont != nil {
		style.Font = s.BoldItalicFont
	}
	return style
}

func (s *htmlStyleStack) addItalic() htmlTextStyle {
	style := s.currentStyle()
	style.Italic = true
	if s.ItalicFont != nil {
		style.Font = s.ItalicFont
	}
	if style.Bold && s.BoldItalicFont != nil {
		style.Font = s.BoldItalicFont
	}
	return style
}

type htmlTableCell struct {
	block *htmlBlock
}

type htmlTableRow struct {
	cells []*htmlTableCell
}

type htmlTable struct {
	table       *Table
	maxColIndex int
	rows        []*htmlTableRow
}

func (t *htmlTable) generateContent() {
	if t.table == nil {
		t.table = newTable(t.maxColIndex)
		for _, row := range t.rows {
			for _, cell := range row.cells {
				c := t.table.NewCell()
				if cell.block != nil {
					c.SetContent(cell.block)
				}
			}
		}
	}
}

// Width returns the width of the Drawable.
func (t *htmlTable) Width() float64 {
	t.generateContent()
	return t.table.Width()
}

// Height returns the height of the Drawable.
func (t *htmlTable) Height() float64 {
	t.generateContent()
	return t.table.Height()
}

// GeneratePageBlocks generates the page blocks.  Multiple blocks are generated if the contents wrap
// over multiple pages. Implements the Drawable interface.
func (t *htmlTable) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	t.generateContent()
	return t.table.GeneratePageBlocks(ctx)
}

type htmlTableStack struct {
	tableStack []*htmlTable
}

func (st *htmlTableStack) createAndPushTable() *htmlTable {
	t := &htmlTable{}
	st.pushTable(t)
	return t
}

func (st *htmlTableStack) currentTable() *htmlTable {
	if len(st.tableStack) == 0 {
		return nil
	}
	return st.tableStack[len(st.tableStack)-1]
}

func (st *htmlTableStack) pushTable(table *htmlTable) {
	st.tableStack = append(st.tableStack, table)
}

func (st *htmlTableStack) popTable() *htmlTable {
	t := st.currentTable()
	if len(st.tableStack) < 1 {
		return t
	}
	st.tableStack = st.tableStack[:len(st.tableStack)-1]
	return t
}

type htmlBlock struct {
	owner      *HtmlParagraph
	parent     *htmlBlock
	tableStack *htmlTableStack
	styleStack *htmlStyleStack

	elements         []VectorDrawable
	currentParagraph *StyledParagraph
}

func newHtmlBlock(parent *htmlBlock) *htmlBlock {
	b := htmlBlock{
		owner:            parent.owner,
		parent:           parent,
		tableStack:       parent.tableStack,
		styleStack:       parent.styleStack,
		elements:         nil,
		currentParagraph: nil,
	}
	return &b
}

var ignoreReplacer = strings.NewReplacer("\r", "", "\n", "", "\t", " ")

func (b *htmlBlock) parseNodeStyle(node *html.Node) htmlTextStyle {
	style := b.styleStack.currentStyle()
	for _, attr := range node.Attr {
		switch attr.Key {
		case "style":
			style.addEmbeddedCSS(node.Data, attr.Val)
		case "align":
			switch attr.Val {
			case "center":
				style.TextAlignment = TextAlignmentCenter
			case "left":
				style.TextAlignment = TextAlignmentLeft
			case "right":
				style.TextAlignment = TextAlignmentRight
			case "justify":
				style.TextAlignment = TextAlignmentJustify
			}
		}
	}
	return style
}

func (b *htmlBlock) processNode(node *html.Node) error {
	newB := b

	switch node.Type {
	case html.TextNode:
		p, created := b.getCurrentOrCreateParagraph()
		if created {
			b.currentParagraph.alignment = b.styleStack.currentStyle().TextAlignment
		}
		text := ignoreReplacer.Replace(node.Data)
		p.Append(text).Style = b.styleStack.currentStyle().TextStyle
		return nil
	case html.ElementNode:
		style := b.parseNodeStyle(node)

		switch node.Data {
		case "style":
			log.Println(node)
			return nil
		case "script":
			return nil
		case "table":
			t := b.tableStack.createAndPushTable()
			b.styleStack.pushStyle(style)
			defer b.styleStack.popStyle()
			b.elements = append(b.elements, t)
			defer b.tableStack.popTable()
		case "tr":
			if t := b.tableStack.currentTable(); t != nil {
				t.rows = append(t.rows, &htmlTableRow{})
			}
		case "td", "th":
			if t := b.tableStack.currentTable(); t != nil && len(t.rows) > 0 {
				b.styleStack.pushStyle(style)
				defer b.styleStack.popStyle()

				row := t.rows[len(t.rows)-1]

				newB = newHtmlBlock(b)
				//b.elements = append(b.elements, newB)

				cell := htmlTableCell{block: newB}
				row.cells = append(row.cells, &cell)
				if l := len(row.cells); l > t.maxColIndex {
					t.maxColIndex = l
				}
			}
			if node.Data == "th" {
				b.styleStack.pushStyle(b.styleStack.addBold())
				defer b.styleStack.popStyle()
			}
		case "p":
			b.styleStack.pushStyle(style)
			defer b.styleStack.popStyle()
			b.currentParagraph = newStyledParagraph(b.styleStack.currentStyle().TextStyle)
			b.currentParagraph.alignment = b.styleStack.currentStyle().TextAlignment
			b.elements = append(b.elements, b.currentParagraph)
		case "br":
			p, created := b.getCurrentOrCreateParagraph()
			if created {
				b.currentParagraph.alignment = b.styleStack.currentStyle().TextAlignment
			}
			p.Append("\n")
		case "b":
			b.styleStack.pushStyle(b.styleStack.addBold())
			defer b.styleStack.popStyle()
		case "i":
			b.styleStack.pushStyle(b.styleStack.addItalic())
			defer b.styleStack.popStyle()
		}
	}

	for next := node.FirstChild; next != nil; next = next.NextSibling {
		if err := newB.processNode(next); err != nil {
			return err
		}
	}
	return nil
}

type HtmlParagraph struct {
	blocks     []*htmlBlock
	tableStack htmlTableStack
	styleStack htmlStyleStack
}

func (b *htmlBlock) getCurrentOrCreateParagraph() (*StyledParagraph, bool) {
	if b.currentParagraph == nil {
		b.currentParagraph = newStyledParagraph(b.styleStack.RegularStyle)
		b.elements = append(b.elements, b.currentParagraph)
		return b.currentParagraph, true
	}
	return b.currentParagraph, false
}

// Width returns the width of the Drawable.
func (b *htmlBlock) Width() float64 {
	var w float64
	for _, e := range b.elements {
		w = math.Max(w, e.Width())
	}
	return w
}

// Height returns the height of the Drawable.
func (b *htmlBlock) Height() float64 {
	var h float64
	for _, e := range b.elements {
		h = math.Max(h, e.Height())
	}
	return h
}

// GeneratePageBlocks generates the page blocks.  Multiple blocks are generated if the contents wrap
// over multiple pages. Implements the Drawable interface.
func (b *htmlBlock) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	var blocks []*Block
	origCtx := ctx
	for _, e := range b.elements {
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
	hp := HtmlParagraph{}
	hp.styleStack.RegularStyle = baseStyle
	return &hp
}

func (h *HtmlParagraph) SetRegularStyle(style TextStyle) {
	h.styleStack.RegularStyle = style
}

func (h *HtmlParagraph) SetRegularFont(font *model.PdfFont) {
	h.styleStack.RegularStyle.Font = font
}

func (h *HtmlParagraph) SetBoldFont(font *model.PdfFont) {
	h.styleStack.BoldFont = font
}

func (h *HtmlParagraph) SetItalicFont(font *model.PdfFont) {
	h.styleStack.ItalicFont = font
}

func (h *HtmlParagraph) SetBoldItalicFont(font *model.PdfFont) {
	h.styleStack.BoldItalicFont = font
}

// Append adds html to paragraph.
func (h *HtmlParagraph) Append(htmlCode string) error {
	doc, err := html.Parse(bytes.NewBufferString(htmlCode))
	if err != nil {
		return err
	}
	newB := htmlBlock{
		owner:      h,
		parent:     nil,
		tableStack: &h.tableStack,
		styleStack: &h.styleStack,
	}
	h.blocks = append(h.blocks, &newB)
	return newB.processNode(doc)
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

// GeneratePageBlocks generates the page blocks.  Multiple blocks are generated if the contents wrap
// over multiple pages. Implements the Drawable interface.
func (h *HtmlParagraph) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	var blocks []*Block
	origCtx := ctx
	for _, e := range h.blocks {
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
