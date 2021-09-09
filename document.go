package gemini

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

func NewDocumentWriter(w io.Writer) *DocumentWriter {
	return &DocumentWriter{
		w: w,
	}
}

type DocumentWriter struct {
	w io.Writer
}

func (dw *DocumentWriter) NewLine() (err error) {
	_, err = io.WriteString(dw.w, "\n")
	return fmt.Errorf("error writing newline: %w", err)
}

func (dw *DocumentWriter) Header1(text string) (err error) {
	if _, err = io.WriteString(dw.w, "# "); err != nil {
		return fmt.Errorf("error writing header1: %w", err)
	}
	if _, err = io.WriteString(dw.w, text); err != nil {
		return fmt.Errorf("error writing header1: %w", err)
	}
	if _, err = io.WriteString(dw.w, "\n"); err != nil {
		return fmt.Errorf("error writing header1: %w", err)
	}
	return nil
}

func (dw *DocumentWriter) Header2(text string) (err error) {
	if _, err = io.WriteString(dw.w, "## "); err != nil {
		return fmt.Errorf("error writing header2: %w", err)
	}
	if _, err = io.WriteString(dw.w, text); err != nil {
		return fmt.Errorf("error writing header2: %w", err)
	}
	if _, err = io.WriteString(dw.w, "\n"); err != nil {
		return fmt.Errorf("error writing header2: %w", err)
	}
	return nil
}

func (dw *DocumentWriter) Quote(text string) (err error) {
	if _, err = io.WriteString(dw.w, "> "); err != nil {
		return fmt.Errorf("error writing quote: %w", err)
	}
	if _, err = io.WriteString(dw.w, text); err != nil {
		return fmt.Errorf("error writing quote: %w", err)
	}
	if _, err = io.WriteString(dw.w, "\n"); err != nil {
		return fmt.Errorf("error writing quote: %w", err)
	}
	return nil
}

func (dw *DocumentWriter) Line(text string) (err error) {
	if _, err = io.WriteString(dw.w, text); err != nil {
		return err
	}
	if _, err = io.WriteString(dw.w, "\n"); err != nil {
		return fmt.Errorf("error writing line: %w", err)
	}
	return nil
}

// DocumentBuilder allows programmatic document creation using the builder pattern.
// DocumentBuilder supports the use of headers and footers, which are combined with the body at build time.
type DocumentBuilder struct {
	header string
	body   *strings.Builder
	footer string
}

// NewDocumentBuilder creates a DocumentBuilder.
func NewDocumentBuilder() *DocumentBuilder {
	builder := new(strings.Builder)
	return &DocumentBuilder{"", builder, ""}
}

// SetHeader sets a document header. The header is written before the document body during `Build()`.
func (doc *DocumentBuilder) SetHeader(header string) {
	doc.header = header
}

// SetFooter sets a document footer. The footer is written after the document body during `Build()`.
func (doc *DocumentBuilder) SetFooter(footer string) {
	doc.footer = footer
}

// AddLine appends a new line to the document. Adds a newline to the end of the line if none is present.
func (doc *DocumentBuilder) AddLine(line string) error {
	_, err := doc.body.WriteString(line)
	if err != nil {
		return errors.Wrap(err, "Error writing to document")
	}

	if !strings.HasSuffix(line, "\n") {
		_, err = doc.body.WriteString("\n")
		if err != nil {
			return errors.Wrap(err, "Error writing to document")
		}
	}

	return nil
}

// AddH1Header appends an H1 (#) header line to the document.
func (doc *DocumentBuilder) AddH1Header(header string) error {
	_, err := doc.body.WriteString("# ")
	if err != nil {
		return errors.Wrap(err, "Error writing to document")
	}
	return doc.AddLine(header)
}

// AddH2Header appends an H2 (##) header line to the document.
func (doc *DocumentBuilder) AddH2Header(header string) error {
	_, err := doc.body.WriteString("## ")
	if err != nil {
		return errors.Wrap(err, "Error writing to document")
	}
	return doc.AddLine(header)
}

// AddH3Header appends an H3 (###) header line to the document.
func (doc *DocumentBuilder) AddH3Header(header string) error {
	_, err := doc.body.WriteString("### ")
	if err != nil {
		return errors.Wrap(err, "Error writing header line to document")
	}
	return doc.AddLine(header)
}

// AddQuote appends a quote line to the document.
func (doc *DocumentBuilder) AddQuote(header string) error {
	_, err := doc.body.WriteString("> ")
	if err != nil {
		return errors.Wrap(err, "Error writing quote to document")
	}
	return doc.AddLine(header)
}

// AddBullet appends an unordered list item to the document.
func (doc *DocumentBuilder) AddBullet(header string) error {
	_, err := doc.body.WriteString("* ")
	if err != nil {
		return errors.Wrap(err, "Error writing bullet to document")
	}
	return doc.AddLine(header)
}

// ToggleFormatting appends a toggle formatting line to the document.
func (doc *DocumentBuilder) ToggleFormatting() error {
	return doc.AddLine("```")
}

// AddLink appends an aliased link line to the document.
func (doc *DocumentBuilder) AddLink(url string, title string) error {
	_, err := doc.body.WriteString("=> ")
	if err != nil {
		return errors.Wrap(err, "Error writing link to document")
	}
	_, err = doc.body.WriteString(url)
	if err != nil {
		return errors.Wrap(err, "Error writing link to document")
	}
	_, err = doc.body.WriteString("\t")
	if err != nil {
		return errors.Wrap(err, "Error writing link to document")
	}
	// AddLine to ensure there is a newline
	return doc.AddLine(title)
}

// AddRawLink appends a link line to the document.
func (doc *DocumentBuilder) AddRawLink(url string) error {
	_, err := doc.body.WriteString("=> ")
	if err != nil {
		return errors.Wrap(err, "Error writing raw link to document")
	}
	err = doc.AddLine(url)
	return err
}

// Build builds the document into a serialized byte slice.
func (doc *DocumentBuilder) Build() ([]byte, error) {
	buf := bytes.Buffer{}

	// Write header
	_, err := buf.WriteString(doc.header)
	if err != nil {
		return nil, errors.Wrap(err, "Error building document header")
	}

	// Write body
	_, err = buf.WriteString(doc.body.String())
	if err != nil {
		return nil, errors.Wrap(err, "Error building document body")
	}

	// Write footer
	_, err = buf.WriteString(doc.footer)
	if err != nil {
		return nil, errors.Wrap(err, "Error building document footer")
	}

	return buf.Bytes(), nil
}
