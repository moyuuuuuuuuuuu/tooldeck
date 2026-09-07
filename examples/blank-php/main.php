<?php
declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use ToolDeck\BlankTool\Application;

try {
    $input = json_decode(stream_get_contents(STDIN), true, 32, JSON_THROW_ON_ERROR);
    if (!is_array($input)) {
        throw new InvalidArgumentException('输入必须是 JSON 对象');
    }
    echo json_encode(
        (new Application())->run($input),
        JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR,
    );
} catch (Throwable $exception) {
    fwrite(STDERR, $exception->getMessage());
    exit(1);
}
