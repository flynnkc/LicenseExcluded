package results

import (
	"encoding/json"
	"sync"
)

type Result struct {
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
	ItemsFound int    `json:"items_found,omitempty"`
	Changes    int    `json:"license_changes,omitempty"`
	sync.Mutex
}

func NewResult() Result {
	return Result{
		ItemsFound: 0,
	}
}

func (r *Result) AddChanges(n int) {
	r.Lock()
	defer r.Unlock()
	r.Changes += n
}

func (r *Result) AddItemsFound(n int) {
	r.Lock()
	defer r.Unlock()
	r.ItemsFound += n
}

func (r *Result) SetMessage(msg string) {
	r.Lock()
	defer r.Unlock()
	r.Message = msg
}

func (r *Result) JsonEncode() string {
	b, err := json.Marshal(r)
	if err != nil {
		return "Error marshaling results into JSON"
	}

	return string(b)
}
