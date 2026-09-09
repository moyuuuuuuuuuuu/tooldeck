<?php
declare(strict_types=1);

namespace ToolDeck\BlankTool;

final class Application implements CancelableToolInterface
{
    /** @param array<string, mixed> $input
     *  @return array<string, mixed>
     */
    public function run(array $input): array
    {
        $text = trim((string) ($input['text'] ?? ''));
        if ($text === '') {
            throw new \InvalidArgumentException('文本内容不能为空');
        }

        // 在这里替换为你的业务逻辑。
        return [
            'message' => 'PHP 空白工具运行成功',
            'text' => $text,
        ];
    }

    /** @param array<string, mixed> $input
     *  @param array<string, mixed> $state
     */
    public function cancel(array $input, array $state): void
    {
        // 用 $state['provider_task_id'] 调用第三方取消接口。
        // 此空白示例没有远程任务，因此无需执行操作。
    }
}
