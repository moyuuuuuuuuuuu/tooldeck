<?php
declare(strict_types=1);

namespace ToolDeck\BlankTool;

interface CancelableToolInterface extends ToolInterface
{
    /**
     * @param array<string, mixed> $input Original run input.
     * @param array<string, mixed> $state State saved by run(), such as a provider task ID.
     */
    public function cancel(array $input, array $state): void;
}
