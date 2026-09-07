import json
import sys
from typing import Any


class Application:
    def run(self, input_data: dict[str, Any]) -> dict[str, Any]:
        text = str(input_data.get("text", "")).strip()
        if not text:
            raise ValueError("文本内容不能为空")

        # 在这里替换为你的业务逻辑。
        return {"message": "Python 空白工具运行成功", "text": text}


def main() -> None:
    try:
        input_data = json.load(sys.stdin)
        if not isinstance(input_data, dict):
            raise ValueError("输入必须是 JSON 对象")
        json.dump(Application().run(input_data), sys.stdout, ensure_ascii=False)
    except (ValueError, json.JSONDecodeError) as error:
        print(str(error), file=sys.stderr, end="")
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
