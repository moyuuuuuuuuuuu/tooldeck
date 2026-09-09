import { Tool } from "./tool.js";

export class Application extends Tool {
  constructor() {
    super();
  }

  run(input) {
    const text = String(input.text ?? "").trim();
    if (!text) throw new Error("文本内容不能为空");

    // 在这里替换为你的业务逻辑。
    return { message: "Node.js 空白工具运行成功", text };
  }

  cancel(input, state) {
    // 使用 state.provider_task_id 调用第三方取消接口。
    // 此空白示例没有远程任务，因此无需执行操作。
  }
}
