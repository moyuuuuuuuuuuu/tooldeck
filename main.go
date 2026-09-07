package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Input struct {
	Text string `json:"text"`
}

type Output struct {
	Message string `json:"message"`
	Text    string `json:"text"`
}

type Application struct{}

func (Application) Run(input Input) (Output, error) {
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return Output{}, errors.New("文本内容不能为空")
	}

	// 在这里替换为你的业务逻辑。
	return Output{Message: "Go 空白工具运行成功", Text: text}, nil
}

func main() {
	var input Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fail(err)
	}
	output, err := (Application{}).Run(input)
	if err != nil {
		fail(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprint(os.Stderr, err.Error())
	os.Exit(1)
}
