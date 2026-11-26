package svncfg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Line struct {
	Text   string
	Ending string
}

type TextDocument struct {
	BOM   []byte
	Lines []Line
}

func ParseDocument(data []byte) *TextDocument {
	doc := &TextDocument{}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		doc.BOM = append([]byte(nil), data[:3]...)
		data = data[3:]
	}
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			doc.Lines = append(doc.Lines, Line{Text: string(data), Ending: ""})
			break
		}
		text := data[:idx]
		ending := "\n"
		if len(text) > 0 && text[len(text)-1] == '\r' {
			text = text[:len(text)-1]
			ending = "\r\n"
		}
		doc.Lines = append(doc.Lines, Line{Text: string(text), Ending: ending})
		data = data[idx+1:]
	}
	return doc
}

func (d *TextDocument) Bytes() []byte {
	var b strings.Builder
	if len(d.BOM) > 0 {
		b.Write(d.BOM)
	}
	for _, line := range d.Lines {
		b.WriteString(line.Text)
		b.WriteString(line.Ending)
	}
	return []byte(b.String())
}

func (d *TextDocument) Newline() string {
	for _, line := range d.Lines {
		if line.Ending != "" {
			return line.Ending
		}
	}
	return "\n"
}

func (d *TextDocument) InsertLine(index int, text string) {
	if index < 0 || index > len(d.Lines) {
		index = len(d.Lines)
	}
	ending := d.Newline()
	if index > 0 && d.Lines[index-1].Ending == "" {
		d.Lines[index-1].Ending = ending
	}
	line := Line{Text: text, Ending: ending}
	d.Lines = append(d.Lines, Line{})
	copy(d.Lines[index+1:], d.Lines[index:])
	d.Lines[index] = line
}

func (d *TextDocument) AppendLine(text string) {
	d.InsertLine(len(d.Lines), text)
}

func (d *TextDocument) RemoveLine(index int) {
	if index < 0 || index >= len(d.Lines) {
		return
	}
	d.Lines = append(d.Lines[:index], d.Lines[index+1:]...)
}

func (d *TextDocument) ReplaceBlock(start, end int, texts []string) {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(d.Lines) {
		end = len(d.Lines)
	}
	ending := d.Newline()
	replacement := make([]Line, 0, len(texts))
	for _, text := range texts {
		replacement = append(replacement, Line{Text: text, Ending: ending})
	}
	combined := make([]Line, 0, len(d.Lines)-(end-start)+len(replacement))
	combined = append(combined, d.Lines[:start]...)
	combined = append(combined, replacement...)
	combined = append(combined, d.Lines[end:]...)
	d.Lines = combined
}

func Revision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func replaceRange(s string, start, end int, replacement string) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	return s[:start] + replacement + s[end:]
}
