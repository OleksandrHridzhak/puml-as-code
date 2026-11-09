package ast

import "yur4uwe/pac/pkg/tokenizer"

type GradientKind byte

const (
	GradientNone       GradientKind = iota
	GradientHorizontal              // -
	GradientVertical                // |
	GradientSlash                   // /
	GradientBackslash               // \
)

type LineStyle byte

const (
	LineNormal LineStyle = iota
	LineBold
	LineDashed
	LineDotted
)

// ColorExpr represents any valid color expression in PlantUML.
// Sealed interface implemented by *ColorRef and *GradientColor.
type ColorExpr interface {
	Span() tokenizer.SourceSpan
	colorExpr()
}

type ColorRef struct {
	Literal   string
	ColorSpan tokenizer.SourceSpan
}

func (s *ColorRef) Span() tokenizer.SourceSpan { return s.ColorSpan }
func (s *ColorRef) colorExpr()                 {}

// GradientColor represents a two-stop color gradient: Start <dir> End.
type GradientColor struct {
	Start     *ColorRef
	End       *ColorRef
	Direction GradientKind
}

func (g *GradientColor) Span() tokenizer.SourceSpan {
	if g == nil {
		return tokenizer.SourceSpan{}
	}
	var startPos, endPos tokenizer.Pos
	if g.Start != nil {
		startPos = g.Start.Span().Start
	}
	if g.End != nil {
		endPos = g.End.Span().End
	}
	return tokenizer.SourceSpan{Start: startPos, End: endPos}
}

func (g *GradientColor) colorExpr() {}

// EntityStyle models inline style annotations on entities:
//
//	#backgroundColor ##[lineStyle]lineColor
//	#[color|back:color];header:color;line:color;line.[bold|dashed|dotted];text:color
type EntityStyle struct {
	Span       tokenizer.SourceSpan `json:",omitempty"`
	Background ColorExpr            `json:",omitempty"`
	Header     ColorExpr            `json:",omitempty"`
	LineColor  ColorExpr            `json:",omitempty"`
	LineStyle  LineStyle            `json:",omitempty"`
	TextColor  ColorExpr            `json:",omitempty"`
}

// PackageStyle models inline style annotations on packages/containers:
//
//	#[color|back:color];line:color;line.[bold|dashed|dotted];text:color
type PackageStyle struct {
	Span       tokenizer.SourceSpan `json:",omitempty"`
	Background ColorExpr            `json:",omitempty"`
	LineColor  ColorExpr            `json:",omitempty"`
	LineStyle  LineStyle            `json:",omitempty"`
	TextColor  ColorExpr            `json:",omitempty"`
}

// RelationshipStyle models inline style annotations on relationships/arrows:
//
//	#line:color;line.[bold|dashed|dotted];text:color
type RelationshipStyle struct {
	Span      tokenizer.SourceSpan `json:",omitempty"`
	LineColor ColorExpr            `json:",omitempty"`
	LineStyle LineStyle            `json:",omitempty"`
	TextColor ColorExpr            `json:",omitempty"`
}

// NoteStyle models inline style annotations on notes:
//
//	#color
type NoteStyle struct {
	Span  tokenizer.SourceSpan `json:",omitempty"`
	Color ColorExpr            `json:",omitempty"`
}
