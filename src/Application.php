<?php
declare(strict_types=1);

namespace ToolDeck\BlankTool;

final class Application
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
}
