package parser

import (
	"regexp"
	"strings"

	"yur4uwe/pac/pkg/parser/ast"
	"yur4uwe/pac/pkg/tokenizer"
	"yur4uwe/pac/pkg/tokenizer/keyword"
)

func (p *Parser) parseVisibilityCommand(tok tokenizer.Token) *ast.VisibilityCommand {
	cmd := &ast.VisibilityCommand{}
	switch keyword.Classify(tok.Literal) {
	case keyword.Hide:
		cmd.Kind = ast.VisibilityCMDHide
	case keyword.Show:
		cmd.Kind = ast.VisibilityCMDShow
	case keyword.Remove:
		cmd.Kind = ast.VisibilityCMDRemove
	case keyword.Restore:
		cmd.Kind = ast.VisibilityCMDRestore
	default:
		panic("visibility command: unreachable")
	}
	cmd.Target = p.stream.ReadUntilNewline()
	if len(cmd.Target) == 0 {
		nextTok := p.stream.PeekTokenAt(0)
		err := NewParserError(nextTok, "empty target for visibility command")
		p.ast.RecordError(
			err, nextTok.Span, cmd,
		)
	}
	return cmd
}

func (p *Parser) parseDiagDirection(tok tokenizer.Token) *ast.DirectionCommand {
	var to string
	switch tok.Literal {
	case "left":
		to = "right"
	case "top":
		to = "bottom"
	}

	expectedSeq := []tokenizer.Token{
		{
			Type:    tokenizer.IDENTIFIER,
			Literal: "to",
		},
		{
			Type:    tokenizer.IDENTIFIER,
			Literal: to,
		},
		{
			Type:    tokenizer.IDENTIFIER,
			Literal: "direction",
		},
	}

	cmd := &ast.DirectionCommand{}

	for _, token := range expectedSeq {
		if _, ok := p.stream.TryConsume(token); ok {
			continue
		}

		nextToken := p.stream.PeekTokenAt(0)
		err := NewParserError(nextToken, "Unexpected diagram direction modifier")
		p.ast.RecordError(
			err, nextToken.Span, cmd,
		)
		p.syncToLineEnd()
		return cmd
	}
	switch tok.Literal {
	case "left":
		cmd.Direction = ast.LeftToRightDirection
	case "top":
		cmd.Direction = ast.TopToBottomDirection
	}
	if !p.stream.AssertType(tokenizer.NEWLINE) {
		nextToken := p.stream.PeekTokenAt(0)
		err := NewParserError(nextToken, "Unexpected tokens after direction command")
		p.ast.RecordError(
			err, nextToken.Span, cmd,
		)
		p.syncToLineEnd()
	}
	return cmd
}

// Scale operands are unsigned decimal literals. Extract them from the source
// line instead of relying on the lexer to split every spelling of a box.
var scaleOperands = regexp.MustCompile(`^([0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:\s*([*x/])\s*([0-9]+(?:\.[0-9]*)?|\.[0-9]+)?)?(?:\s+(width|height)\b)?`)

func (p *Parser) parseScale(start tokenizer.Token) *ast.ScaleCommand {
	cmd := &ast.ScaleCommand{}
	_, cmd.IsMax = p.stream.TryConsume(amb(tokenizer.IDENTIFIER, "max"))
	tokens := p.syncToLineEnd()
	span := p.Span(p.Mark(start))
	fail := func(message string) *ast.ScaleCommand {
		p.ast.RecordError(NewParserError(start, message), span, cmd)
		return cmd
	}
	src := p.stream.SliceInputEnclosingTokens(tokens...)
	match := scaleOperands.FindStringSubmatch(src)
	if match == nil {
		return fail("expected a decimal number after scale")
	}
	// An adjacent 0x prefix is reserved for hexadecimal numbers, not boxes.
	if strings.HasPrefix(src, "0x") && len(tokens) > 0 && strings.HasPrefix(tokens[0].Literal, "0x") {
		return fail("hexadecimal numbers are not supported in scale")
	}
	cmd.Lhs, cmd.Sep, cmd.Rhs, cmd.Unit = match[1], match[2], match[3], match[4]
	if strings.TrimSpace(src[len(match[0]):]) != "" {
		return fail("unexpected tokens after scale operands")
	}
	decimal := strings.Contains(cmd.Lhs, ".")
	switch {
	case cmd.Sep != "" && (cmd.Rhs == "" || decimal || strings.Contains(cmd.Rhs, ".")):
		return fail("expected decimal integers on both sides of the scale separator")
	case cmd.Sep != "" && cmd.Unit != "":
		return fail("cannot specify a unit with a box or fraction")
	case cmd.IsMax && (decimal || cmd.Sep == "/" || cmd.Sep == "" && cmd.Unit == ""):
		return fail("scale max requires an integer width, height, or box")
	}
	// Preserve the established AST representation of a bare full decimal.
	if cmd.Unit == "" && cmd.Sep == "" {
		if lhs, rhs, ok := strings.Cut(cmd.Lhs, "."); ok && lhs != "" && rhs != "" {
			cmd.Lhs, cmd.Sep, cmd.Rhs = lhs, ".", rhs
		}
	}
	return cmd
}

func (p *Parser) parseSetCommand() *ast.SetCommand {
	keyTok := p.stream.Emit() // consume "separator"

	toks := p.stream.ConsumeUntilType(tokenizer.NEWLINE)
	var sb strings.Builder
	for _, t := range toks {
		sb.WriteString(t.Literal)
	}
	directiveVal := sb.String()
	if keyTok.Literal == "separator" {
		if directiveVal == "none" {
			p.stream.PackageSeparator = ""
		} else {
			p.stream.PackageSeparator = directiveVal
		}
	}

	return &ast.SetCommand{
		Key:   keyTok.Literal,
		Value: directiveVal,
	}
}

func parseLayoutAlignmentToken(tok tokenizer.Token, blockKind ast.TextBlockKind) (horiz string, vert string, isAlign bool) {
	switch strings.ToLower(tok.Literal) {
	case "left", "right", "center":
		return strings.ToLower(tok.Literal), "", true
	case "top", "bottom":
		isAlign = true
		if blockKind != ast.BlockLegend {
			isAlign = false
		}
		return "", strings.ToLower(tok.Literal), isAlign
	default:
		return "", "", false
	}
}

func (p *Parser) parseLayoutStatement(kwTok tokenizer.Token, prefixAlignment *tokenizer.Token) *ast.TextBlock {
	blockKind := mapKwTokToTextBlockKind(keyword.Classify(kwTok.Literal))

	block := &ast.TextBlock{
		Kind: blockKind,
	}

	// Process prefix alignment if provided (e.g. "left header", "center footer")
	if prefixAlignment != nil && blockKind != ast.BlockTitle {
		h, v, isAlign := parseLayoutAlignmentToken(*prefixAlignment, blockKind)
		if !isAlign {
			var err error
			if len(v) > 0 {
				err = NewParserError(*prefixAlignment, "Vertical alignment only supported for legend")
			} else {
				err = NewParserError(*prefixAlignment, "Invalid alignment")
			}
			p.ast.RecordError(err, *&prefixAlignment.Span, block)
		}
		block.HorizontalAlignment = h
		block.VerticalAlignment = v
	} else if prefixAlignment != nil && blockKind == ast.BlockTitle {
		err := NewParserError(*prefixAlignment, "Title alignment not supported")
		p.ast.RecordError(err, *&prefixAlignment.Span, block)
	}

	// Consume any trailing alignment modifiers on the opener line (e.g. "legend top left", "header center")
	for !p.stream.AssertAnyType(tokenizer.NEWLINE, tokenizer.EOF) {
		peekTok := p.stream.PeekTokenAt(0)
		h, v, isAlign := parseLayoutAlignmentToken(peekTok, blockKind)
		if !isAlign {
			break
		}
		if blockKind == ast.BlockTitle {
			break // For title, any tokens on the same line are title text
		}
		if h != "" {
			if block.HorizontalAlignment != "" {
				err := NewParserError(peekTok, "Horizontal alignment already set")
				p.ast.RecordError(err, peekTok.Span, block)
			} else {
				block.HorizontalAlignment = h
			}
			p.stream.Emit()
		} else if v != "" {
			if block.VerticalAlignment != "" {
				err := NewParserError(peekTok, "Vertical alignment already set")
				p.ast.RecordError(err, peekTok.Span, block)
			} else {
				block.VerticalAlignment = v
			}
			p.stream.Emit()
		}
	}

	lineContentToks := p.stream.ConsumeUntilType(tokenizer.NEWLINE)

	var body string
	if len(lineContentToks) == 0 {
		// --- MULTI-LINE BLOCK FORM ---
		var err error
		body, err = p.stream.ConsumeTextBlock("end", kwTok.Literal)
		if err != nil {
			err := ast.NewUnclosedBlockError(kwTok.Literal, "end "+kwTok.Literal)
			p.ast.RecordError(err, kwTok.Span, block)
			return block
		}
	} else {
		// --- SINGLE-LINE INLINE FORM ---
		body = p.stream.SliceInputEnclosingTokens(lineContentToks...)
	}

	block.Text = body
	if blockKind == ast.BlockTitle {
		p.ast.Title = block.Text
	}
	return block
}
