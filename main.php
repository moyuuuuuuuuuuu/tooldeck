<?php
declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use ToolDeck\BlankTool\Application;

try {
    $input = json_decode(stream_get_contents(STDIN), true, 32, JSON_THROW_ON_ERROR);
    if (!is_array($input)) {
        throw new InvalidArgumentException('输入必须是 JSON 对象');
    }
    $application = new Application();
    if (getenv('TOOLDECK_ACTION') === 'cancel') {
        $stateFile = (string) getenv('TOOLDECK_STATE_FILE');
        $state = is_file($stateFile)
            ? json_decode((string) file_get_contents($stateFile), true, 32, JSON_THROW_ON_ERROR)
            : [];
        $application->cancel($input, is_array($state) ? $state : []);
        exit(0);
    }

    echo json_encode(
        $application->run($input),
        JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR,
    );
} catch (Throwable $exception) {
    fwrite(STDERR, $exception->getMessage());
    exit(1);
}
