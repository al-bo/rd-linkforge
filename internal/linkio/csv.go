package linkio

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
)

// WriteCSV writes the header id,code,target,owner_id,created_at,expires_at and one row per link.
// Times are RFC 3339 in UTC; a link that never expires has an empty expires_at.
func WriteCSV(w io.Writer, links []link.Link) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"id", "code", "target", "owner_id", "created_at", "expires_at"}); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	for i, l := range links {
		expiresAt := ""
		if !l.ExpiresAt.IsZero() {
			expiresAt = l.ExpiresAt.UTC().Format(time.RFC3339)
		}
		row := []string{
			strconv.FormatUint(l.ID, 10),
			l.Code,
			l.Target,
			l.OwnerID,
			l.CreatedAt.UTC().Format(time.RFC3339),
			expiresAt,
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write CSV record %d: %w", i+1, err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV: %w", err)
	}
	return nil
}
