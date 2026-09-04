package challenge

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maxSolutionBodyBytes = 4096

type solutionBody struct {
	Nonce   string `json:"nonce"`
	Counter uint64 `json:"counter"`
}

// ReadSolution extracts a PoW solution from JSON or form bodies.
func ReadSolution(r *http.Request) (nonce, counter string, jsonRequest bool) {
	if r == nil {
		return "", "", false
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		var body solutionBody
		decoder := json.NewDecoder(io.LimitReader(r.Body, maxSolutionBodyBytes))
		if err := decoder.Decode(&body); err != nil {
			return "", "", true
		}
		return body.Nonce, strconv.FormatUint(body.Counter, 10), true
	}
	_ = r.ParseForm()
	nonce = r.FormValue("nonce")
	if nonce == "" {
		nonce = r.FormValue("challenge_id")
	}
	counter = r.FormValue("counter")
	if counter == "" {
		counter = r.FormValue("answer")
	}
	return nonce, counter, false
}

func WantsJSON(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json")
}
