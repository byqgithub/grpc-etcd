package utils

import (
	"encoding/json"
	"fmt"
)

type ServiceInfo struct {
	Name    string `json:"Name"`
	Addr    string `json:"addr"`
	Weight  string `json:"weight"`
	Version string `json:"version"`
	TTL     int64  `json:"ttl"`
}

func (si *ServiceInfo) ServerEtecKey() string {
	return fmt.Sprintf("/%s/%s", si.Name, si.Version)
}

func (si *ServiceInfo) Encoder() (string, error) {
	if out, err := json.Marshal(si); err != nil {
		fmt.Printf("Service info json encode error: %v\n", err)
		return "", err
	} else {
		return string(out), nil
	}
}

func ParseValue(value []byte) (ServiceInfo, error) {
	info := ServiceInfo{}
	if err := json.Unmarshal(value, &info); err != nil {
		fmt.Printf("Json parse error: %v\n", err)
		return info, err
	}
	return info, nil
}
