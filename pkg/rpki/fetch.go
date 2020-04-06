package rpki

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Export struct {
	ROAs []ROA `json:"roas"`
}

type ROA struct {
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"maxLength"`
	ASN       string `json:"asn"`
	TA        string `json:"ta"`
}

func Fetch(ctx context.Context, src string) ([]ROA, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", src, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext(): %s: %w", src, err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.Client(): %s: %w", src, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll(): %s: %w", src, err)
	}

	var export Export
	if err := json.Unmarshal(body, &export); err != nil {
		return nil, fmt.Errorf("json.Unmarshal(): %s: %w", src, err)
	}

	return export.ROAs, nil
}
