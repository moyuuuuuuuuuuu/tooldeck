<?php
declare(strict_types=1);

namespace ToolDeck\BlankTool;

interface ToolInterface
{
    /** @param array<string, mixed> $input
     *  @return array<string, mixed>
     */
    public function run(array $input): array;
}
