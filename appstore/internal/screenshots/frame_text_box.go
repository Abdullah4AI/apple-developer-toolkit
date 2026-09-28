package screenshots

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
)

// DefaultTextBoxColor is the fill used behind captions when a text box is
// enabled without an explicit color: 60% black, legible under the default
// white title and gray subtitle.
const DefaultTextBoxColor = "#00000099"

// textBoxPaddingFrac scales the default box padding from each text item's font
// size, so a box keeps its proportions on phone, tablet, watch, TV and Mac.
const textBoxPaddingFrac = 0.25

// koubouHexColorPattern mirrors HEX_COLOR_PATTERN in Koubou 0.20.0's
// config.py: #RGB, #RRGGBB, or #RRGGBBAA.
var koubouHexColorPattern = regexp.MustCompile(`^#(?:[0-9A-Fa-f]{3}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

// TextBoxOptions draws a filled box behind each title and subtitle overlay.
type TextBoxOptions struct {
	Color   string // #RGB, #RRGGBB, or #RRGGBBAA; empty uses DefaultTextBoxColor
	Padding *int   // pixels; nil scales with each item's font size
	Radius  *int   // corner radius in pixels; nil lets Koubou use the padding
}

// koubouTextBox is Koubou 0.20.0's TextBoxConfig.
type koubouTextBox struct {
	Level        string `yaml:"level"`
	Type         string `yaml:"type"`
	Color        string `yaml:"color"`
	Padding      int    `yaml:"padding"`
	CornerRadius *int   `yaml:"corner_radius,omitempty"`
}

// ValidateTextBoxColor accepts the hex forms Koubou's TextBoxConfig accepts.
func ValidateTextBoxColor(color string) error {
	if !koubouHexColorPattern.MatchString(color) {
		return fmt.Errorf("must be a hex color in the form #RGB, #RRGGBB, or #RRGGBBAA, got %q", color)
	}
	return nil
}

// validate checks the options before Koubou runs.
func (box *TextBoxOptions) validate() error {
	if box == nil {
		return nil
	}
	if box.Color != "" {
		if err := ValidateTextBoxColor(box.Color); err != nil {
			return fmt.Errorf("text box color %w", err)
		}
	}
	if box.Padding != nil && *box.Padding < 0 {
		return fmt.Errorf("text box padding must be >= 0")
	}
	if box.Radius != nil && *box.Radius < 0 {
		return fmt.Errorf("text box radius must be >= 0")
	}
	return nil
}

// fingerprint is the resume key for the box settings; empty when disabled.
func (box *TextBoxOptions) fingerprint() string {
	if box == nil {
		return ""
	}
	optional := func(value *int) string {
		if value == nil {
			return ""
		}
		return strconv.Itoa(*value)
	}
	return strings.Join([]string{"on", box.Color, optional(box.Padding), optional(box.Radius)}, "|")
}

// koubouBox renders the box for a text item of the given font size.
func (box *TextBoxOptions) koubouBox(fontSize int) *koubouTextBox {
	if box == nil {
		return nil
	}
	color := box.Color
	if color == "" {
		color = DefaultTextBoxColor
	}
	padding := int(math.Round(float64(fontSize) * textBoxPaddingFrac))
	if box.Padding != nil {
		padding = *box.Padding
	}
	var radius *int
	if box.Radius != nil {
		value := *box.Radius
		radius = &value
	}
	return &koubouTextBox{
		Level:        "paragraph",
		Type:         "rounded",
		Color:        color,
		Padding:      padding,
		CornerRadius: radius,
	}
}

// OverlayTextBox returns the text box an --overlay-config entry enables, or
// nil when the entry leaves it off.
func OverlayTextBox(entry OverlayEntry) *TextBoxOptions {
	if !entry.TextBox {
		return nil
	}
	return &TextBoxOptions{
		Color:   strings.TrimSpace(entry.TextBoxColor),
		Padding: entry.TextBoxPadding,
		Radius:  entry.TextBoxRadius,
	}
}

// ValidateOverlayTextBoxes checks every entry's text box keys. flagEnabled
// reports whether --text-box turns the box on for every input, which lets an
// entry set box details without its own textBox: true.
func ValidateOverlayTextBoxes(config OverlayConfig, flagEnabled bool) error {
	check := func(name string, entry OverlayEntry) error {
		color := strings.TrimSpace(entry.TextBoxColor)
		if color != "" {
			if err := ValidateTextBoxColor(color); err != nil {
				return fmt.Errorf("overlay config %s.textBoxColor %w", name, err)
			}
		}
		if entry.TextBoxPadding != nil && *entry.TextBoxPadding < 0 {
			return fmt.Errorf("overlay config %s.textBoxPadding must be >= 0", name)
		}
		if entry.TextBoxRadius != nil && *entry.TextBoxRadius < 0 {
			return fmt.Errorf("overlay config %s.textBoxRadius must be >= 0", name)
		}
		if entry.TextBox || flagEnabled {
			return nil
		}
		switch {
		case color != "":
			return fmt.Errorf("overlay config %s sets textBoxColor without textBox: true", name)
		case entry.TextBoxPadding != nil:
			return fmt.Errorf("overlay config %s sets textBoxPadding without textBox: true", name)
		case entry.TextBoxRadius != nil:
			return fmt.Errorf("overlay config %s sets textBoxRadius without textBox: true", name)
		}
		return nil
	}
	if err := check("default", config.Default); err != nil {
		return err
	}
	for index, entry := range config.Data {
		if err := check(fmt.Sprintf("data[%d]", index), entry); err != nil {
			return err
		}
	}
	return nil
}

// OverlayEnablesTextBox reports whether any overlay entry turns the box on.
func OverlayEnablesTextBox(config OverlayConfig) bool {
	if config.Default.TextBox {
		return true
	}
	for _, entry := range config.Data {
		if entry.TextBox {
			return true
		}
	}
	return false
}

// maxFontFileBytes bounds a --font file; large CJK .ttc collections stay well
// below it.
const maxFontFileBytes = 64 << 20

// fontFileExtensions are the font file suffixes Koubou 0.20.0 resolves as
// paths (resolve_font_family in generator.py).
var fontFileExtensions = []string{".ttf", ".otf", ".ttc"}

// FontFile is a font read by asc and copied into the private Koubou work root
// so the renderer never reads the operator's path.
type FontFile struct {
	Ext  string // lower-case .ttf, .otf, or .ttc
	Data []byte
}

// Hash returns the SHA-256 digest of the font bytes.
func (font *FontFile) Hash() string {
	if font == nil {
		return ""
	}
	sum := sha256.Sum256(font.Data)
	return hex.EncodeToString(sum[:])
}

// workName is the font's file name inside the Koubou work root. Koubou
// resolves it relative to the generated YAML's directory.
func (font *FontFile) workName() string { return "font" + font.Ext }

// IsFontFilePath reports whether a --font value names a font file rather than
// an installed family: it ends in a font extension or contains a path
// separator, the same test Koubou applies.
func IsFontFilePath(value string) bool {
	if strings.ContainsAny(value, `/\`) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(value))
	for _, allowed := range fontFileExtensions {
		if ext == allowed {
			return true
		}
	}
	return false
}

// LoadFontFile reads a .ttf, .otf, or .ttc file without following a symlink at
// the final path and checks its font signature.
func LoadFontFile(path string) (*FontFile, error) {
	ext := strings.ToLower(filepath.Ext(path))
	supported := false
	for _, allowed := range fontFileExtensions {
		supported = supported || ext == allowed
	}
	if !supported {
		return nil, fmt.Errorf("font file %q must end in .ttf, .otf, or .ttc", path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	defer root.Close()
	data, err := root.ReadFileLimited(filepath.Base(absolute), maxFontFileBytes)
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	if !isParseableFont(data) {
		return nil, fmt.Errorf("font file %q is not a TrueType or OpenType font", path)
	}
	return &FontFile{Ext: ext, Data: data}, nil
}

// maxFontCollectionFonts bounds the font count read from a TTC header.
const maxFontCollectionFonts = 4096

// isParseableFont checks the sfnt structure of a TrueType, OpenType CFF, Apple
// TrueType, or TrueType collection file: every table directory lies within
// the file, every table lies within the file, and each font has the cmap and
// head tables a renderer needs. It rejects renamed or truncated files before
// Koubou runs.
func isParseableFont(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	if string(data[:4]) != "ttcf" {
		return isParseableSFNT(data, 0)
	}
	count := binary.BigEndian.Uint32(data[8:12])
	if count == 0 || count > maxFontCollectionFonts || uint64(len(data)) < 12+4*uint64(count) {
		return false
	}
	for index := uint64(0); index < uint64(count); index++ {
		offset := binary.BigEndian.Uint32(data[12+4*index:])
		if !isParseableSFNT(data, uint64(offset)) {
			return false
		}
	}
	return true
}

func isParseableSFNT(data []byte, offset uint64) bool {
	size := uint64(len(data))
	if offset+12 > size {
		return false
	}
	switch string(data[offset : offset+4]) {
	case "\x00\x01\x00\x00", "OTTO", "true":
	default:
		return false
	}
	tables := uint64(binary.BigEndian.Uint16(data[offset+4:]))
	directoryEnd := offset + 12 + 16*tables
	if tables == 0 || directoryEnd > size {
		return false
	}
	hasCmap, hasHead := false, false
	for record := offset + 12; record < directoryEnd; record += 16 {
		tableOffset := uint64(binary.BigEndian.Uint32(data[record+8:]))
		tableLength := uint64(binary.BigEndian.Uint32(data[record+12:]))
		if tableOffset+tableLength > size {
			return false
		}
		switch string(data[record : record+4]) {
		case "cmap":
			hasCmap = true
		case "head":
			hasHead = true
		}
	}
	return hasCmap && hasHead
}

// TextBoxFingerprint is the resume key for canvas's text box; empty when none.
func TextBoxFingerprint(canvas *CanvasOptions) string {
	if canvas == nil {
		return ""
	}
	return canvas.TextBox.fingerprint()
}
