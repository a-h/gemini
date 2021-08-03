package gemini

// DocumentBuilder allows programatic document creation using the builder pattern.
//
// Example:
// ```
// document := NewDocumentBuilder()
// document.AddH1Header("Hello world!")
// document.AddLine("Reasons to use this builder:")
// document.AddBullet("It's easy to use.")
// document.AddBullet("it's easy to grok the code!")
// document.AddLink("/about", "click here for more!")
//
// responseWriter.Write(document.Build())
// ```
type DocumentBuilder struct {
	body string
}

// Instantiate a new DocumentBuilder.
func NewDocumentBuilder() DocumentBuilder {
	return DocumentBuilder{}
}

// Add a new line to the document. Adds a newline to the end of the line if none is present.
func (self *DocumentBuilder) AddLine(line string) {
	// Insure lines are always newline terminated
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line += "\n"
	}

	self.body += line
}

// Add an H1 (#) header line to the document.
func (self *DocumentBuilder) AddH1Header(header string) {
	self.AddLine("# " + header)
}

// Add an H2 (##) header line to the document.
func (self *DocumentBuilder) AddH2Header(header string) {
	self.AddLine("## " + header)
}

// Add an H3 (###) header line to the document.
func (self *DocumentBuilder) AddH3Header(header string) {
	self.AddLine("### " + header)
}

// Add a quote line to the document.
func (self *DocumentBuilder) AddQuote(header string) {
	self.AddLine("> " + header)
}

// Add an unordered list item to the document.
func (self *DocumentBuilder) AddBullet(header string) {
	self.AddLine("* " + header)
}

// Add a toggle formatting line to the document.
func (self *DocumentBuilder) ToggleFormatting() {
	self.AddLine("```")
}

// Add an aliased link line to the document.
func (self *DocumentBuilder) AddLink(url string, title string) {
	self.AddLine("=> " + url + "\t" + title)
}

// Add a link line to the document.
func (self *DocumentBuilder) AddRawLink(url string) {
	self.AddLine("=> " + url)
}

// Build the document into a serialized byte slice.
func (self *DocumentBuilder) Build() []byte {
	return []byte(self.body)
}
