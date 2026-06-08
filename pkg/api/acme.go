package api

import "fmt"

type EAB struct {
	KeyID  string `json:"key_id"`
	MacKey string `json:"mac_key"`
}

func (r *API) GoogleEAB() (*EAB, error) {
	resp, err := r.client.R().SetResult(&Response{}).Get("/acme/googleEAB")
	if err != nil {
		return nil, err
	}
	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf("failed to get google eab: %s", resp.String())
	}

	eab, err := getResponseData[EAB](resp)
	if err != nil {
		return nil, err
	}

	return eab, nil
}
