package notify

import (
	"fmt"
	"strings"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

func TestFormatNodeCountByStateIncludesEveryState(t *testing.T) {
	got := formatNodeCountByState(1, 2, 3, 4, 5, 6)
	want := fmt.Sprintf(i18n.Trans("High=%d,Low=%d,Warn=%d,Normal=%d,Repair=%d,Other=%d"), 1, 2, 3, 4, 5, 6)
	if got != want || strings.Contains(got, "%!") {
		t.Fatalf("formatNodeCountByState() = %q, want %q", got, want)
	}
}
