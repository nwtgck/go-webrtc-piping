package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/pflag"
)

type JsonFlag struct {
	Value interface{}
}

var _ pflag.Value = (*JsonFlag)(nil)

func (j *JsonFlag) Set(s string) error {
	return json.Unmarshal([]byte(s), j.Value)
}

func (j *JsonFlag) Type() string {
	return "json"
}

func (j *JsonFlag) String() string {
	bs, err := json.Marshal(j.Value)
	if err != nil {
		return fmt.Sprintf("%v", j.Value)
	}
	return string(bs)
}
