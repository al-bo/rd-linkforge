package linkio

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/al-bo/rd-linkforge/internal/link"
)

const header string = "ID,Code,Target,OwnerID,CreatedAT,ExpiresAT"
const newline string = "\n"

var NoEmptyArray = errors.New("Convert to csv: no empty array")

// WriteCSV writes links as CSV to w.
func WriteCSV(w io.Writer, links []link.Link) error {
	if len(links) == 0 {
		return NoEmptyArray
	}
	var builder strings.Builder
	builder.WriteString(header)

	for _, l := range links {
		if _, err := fmt.Fprintf(&builder, "%v%v,%v,%v,%v,%v,%v", newline, l.ID, l.Code, l.Target, l.OwnerID, l.CreatedAt, l.ExpiresAt); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}

	if _, err := io.WriteString(w, builder.String()); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}
	return nil
}
