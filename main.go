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

type State map[string]any

type Tool interface {
	Run(Input) (Output, error)
	Cancel(Input, State) error
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

func (Application) Cancel(input Input, state State) error {
	// 使用 state["provider_task_id"] 调用第三方取消接口。
	// 此空白示例没有远程任务，因此无需执行操作。
	return nil
}

func readState() (State, error) {
	path := os.Getenv("TOOLDECK_STATE_FILE")
	if path == "" {
		return State{}, nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	state := State{}
	if err = json.NewDecoder(file).Decode(&state); err != nil {
		return nil, err
	}
	return state, nil
}

func main() {
	var input Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fail(err)
	}
	var application Tool = Application{}
	if os.Getenv("TOOLDECK_ACTION") == "cancel" {
		state, err := readState()
		if err != nil {
			fail(err)
		}
		if err = application.Cancel(input, state); err != nil {
			fail(err)
		}
		return
	}
	output, err := application.Run(input)
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
