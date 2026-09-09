import json
import os
import sys
from abc import ABC, abstractmethod
from typing import Any


class ToolInterface(ABC):
    @abstractmethod
    def run(self, input_data: dict[str, Any]) -> dict[str, Any]:
        raise NotImplementedError

    @abstractmethod
    def cancel(self, input_data: dict[str, Any], state: dict[str, Any]) -> None:
        raise NotImplementedError


class Application(ToolInterface):
    def run(self, input_data: dict[str, Any]) -> dict[str, Any]:
        text = str(input_data.get("text", "")).strip()
        if not text:
            raise ValueError("文本内容不能为空")

        # 在这里替换为你的业务逻辑。
        return {"message": "Python 空白工具运行成功", "text": text}

    def cancel(self, input_data: dict[str, Any], state: dict[str, Any]) -> None:
        # 使用 state["provider_task_id"] 调用第三方取消接口。
        # 此空白示例没有远程任务，因此无需执行操作。
        return None


def read_state() -> dict[str, Any]:
    state_file = os.environ.get("TOOLDECK_STATE_FILE", "")
    if not state_file or not os.path.isfile(state_file):
        return {}
    with open(state_file, encoding="utf-8") as file:
        state = json.load(file)
    if not isinstance(state, dict):
        raise ValueError("任务状态必须是 JSON 对象")
    return state


def main() -> None:
    try:
        input_data = json.load(sys.stdin)
        if not isinstance(input_data, dict):
            raise ValueError("输入必须是 JSON 对象")
        application = Application()
        if os.environ.get("TOOLDECK_ACTION") == "cancel":
            application.cancel(input_data, read_state())
        else:
            json.dump(application.run(input_data), sys.stdout, ensure_ascii=False)
    except (ValueError, json.JSONDecodeError) as error:
        print(str(error), file=sys.stderr, end="")
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
