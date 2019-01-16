package grafana


import (
	"encoding/json"
	"fmt"
)

// GetAllDatasources loads all datasources.
// It reflects GET /api/datasources API call.
func (r *Client) GetAllFolders() ([]Folder, error) {
	var (
		raw  []byte
		ds   []Folder
		code int
		err  error
	)
	if raw, code, err = r.get("api/folders", nil); err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("HTTP error %d: returns %s", code, raw)
	}
	err = json.Unmarshal(raw, &ds)
	return ds, err
}

func (r *Client) GetFolderByTitle(title string) (*Folder, error) {
	folders, err := r.GetAllFolders()
	if err != nil {
		return nil,err
	}
	for _, folder := range folders {
		if (folder.Title == title) {
			return &folder, nil
		}
	}
	return nil, nil
}


// CreateDatasource creates a new datasource.
// It reflects POST /api/datasources API call.
func (r *Client) CreateFolder(f Folder) (StatusMessage, error) {
	var (
		raw  []byte
		resp StatusMessage
		err  error
	)
	if raw, err = json.Marshal(f); err != nil {
		return StatusMessage{}, err
	}
	if raw, _, err = r.post("api/folders", nil, raw); err != nil {
		return StatusMessage{}, err
	}
	if err = json.Unmarshal(raw, &resp); err != nil {
		return StatusMessage{}, err
	}
	return resp, nil
}
