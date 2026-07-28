package collection

import (
	"encoding/json"
	"func/pkg/logging"
	"log/slog"
	"os"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/resourcesearch"
)

type SearchCollection struct {
	Items map[string]resourcesearch.ResourceSummaryCollection
	sync.Mutex
}

var logger *slog.Logger = logging.NewLogger(os.Getenv("LOG_LEVEL"))

func NewSearchCollection() *SearchCollection {
	sc := SearchCollection{
		Items: make(map[string]resourcesearch.ResourceSummaryCollection),
	}

	return &sc
}

func (sc *SearchCollection) JsonEncode() string {
	b, err := json.Marshal(&sc)
	if err != nil {
		logger.Error("error marshalling search collection into json", "error", err)
		return "Err"
	}

	return string(b)
}
