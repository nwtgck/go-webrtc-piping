package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/pflag"
)

type JSONFlag struct {
	Value interface{}
}

var _ pflag.Value = (*JSONFlag)(nil)

func (j *JSONFlag) Set(s string) error {
	return json.Unmarshal([]byte(s), j.Value)
}

func (j *JSONFlag) Type() string {
	return "json"
}

func (j *JSONFlag) String() string {
	bs, err := json.Marshal(j.Value)
	if err != nil {
		return fmt.Sprintf("%v", j.Value)
	}
	return string(bs)
}
