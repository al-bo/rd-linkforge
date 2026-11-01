package link

import (
	"errors"
	"fmt"
	"strings"
)

const header string = "ID,Code,Target,OwnerID,CreatedAT,ExpiresAT"
const newline string = "\n"

var NoEmptyArray = errors.New("Convert to csv: no empty array")

func ListToCSV(arr []Link) (string, error) {
	if len(arr) == 0 {
		return "", NoEmptyArray
	}
	var builder strings.Builder
	builder.WriteString(header)

	for _, link := range arr {
		if _, err := fmt.Fprintf(&builder, "%v%v,%v,%v,%v,%v,%v", newline, link.ID, link.Code, link.Target, link.OwnerID, link.CreatedAt, link.ExpiresAt); err != nil {
			return "", fmt.Errorf("write CSV row: %w", err)
		}
	}

	return builder.String(), nil
}
