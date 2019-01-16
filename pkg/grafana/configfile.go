package grafana

import (
	"errors"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type DatasourceConfigFile struct {
	ApiVersion        int          `yaml:"apiVersion,omitempty"`
	DeleteDatasources []Datasource `yaml:"deleteDatasources,omitempty"`
	Datasources       []Datasource `yaml:"datasources,omitempty"`
}

func GetGrafanaConfigObjectFromString(source string) (*DatasourceConfigFile, *Board, error) {
	var (
		err   error
		ds    *DatasourceConfigFile
		board *Board
	)
	// try datasource first
	if strings.Contains(source, "apiVersion:") {
		ds, err = DatasourceConfigFileFromString(source)
	} else {
		board, err = BoardFromString(source)
	}

	if board == nil && ds == nil {
		err = errors.New("Not board nor datasource")
	}
	return ds, board, err
}

func DatasourceConfigFileFromString(source string) (*DatasourceConfigFile, error) {
	result := DatasourceConfigFile{}
	err := yaml.Unmarshal([]byte(source), &result)
	return &result, err
}

func (f *DatasourceConfigFile) ToYaml() ([]byte, error) {
	raw, err := yaml.Marshal(f)
	if err != nil {
		return nil, err
	}
	return raw, err
}
