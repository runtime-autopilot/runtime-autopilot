<?php

declare(strict_types=1);

namespace RuntimeAutopilot;

use RuntimeException;

/**
 * Runs the runtime-autopilot binary or falls back to an HTTP endpoint.
 *
 * Resolution order:
 *   1. HTTP GET $RUNTIME_AUTOPILOT_URL (if the env var is set)
 *   2. Shell exec of the binary located at $RUNTIME_AUTOPILOT_BIN
 *      (defaults to "runtime-autopilot" on $PATH)
 */
final class Probe
{
    private const DEFAULT_BINARY = 'runtime-autopilot';

    /**
     * Detect the RuntimeProfile. Returns null and emits a warning when the
     * binary cannot be found and no URL is configured.
     */
    public static function detect(): ?RuntimeProfile
    {
        try {
            $json = self::fetchJson();
        } catch (\Throwable $e) {
            trigger_error(
                'runtime-autopilot: unable to detect profile — ' . $e->getMessage() . '. Skipping auto-configuration.',
                E_USER_WARNING,
            );
            return null;
        }

        $data = json_decode($json, associative: true, flags: JSON_THROW_ON_ERROR);
        if (!is_array($data)) {
            trigger_error('runtime-autopilot: unexpected JSON response. Skipping.', E_USER_WARNING);
            return null;
        }

        return RuntimeProfile::fromArray($data);
    }

    /**
     * Fetch raw JSON from either the HTTP endpoint or the binary.
     *
     * @throws RuntimeException
     */
    private static function fetchJson(): string
    {
        $url = getenv('RUNTIME_AUTOPILOT_URL');
        if ($url !== false && $url !== '') {
            return self::fetchHttp($url);
        }

        return self::fetchBinary();
    }

    /** @throws RuntimeException */
    private static function fetchHttp(string $url): string
    {
        $ctx = stream_context_create(['http' => ['timeout' => 2]]);
        $body = @file_get_contents($url, context: $ctx);
        if ($body === false) {
            throw new RuntimeException("HTTP request to {$url} failed");
        }
        return $body;
    }

    /** @throws RuntimeException */
    private static function fetchBinary(): string
    {
        $bin = getenv('RUNTIME_AUTOPILOT_BIN');
        if ($bin === false || $bin === '') {
            $bin = self::DEFAULT_BINARY;
        }

        $descriptors = [
            0 => ['pipe', 'r'],  // stdin
            1 => ['pipe', 'w'],  // stdout
            2 => ['pipe', 'w'],  // stderr
        ];

        $process = @proc_open(
            escapeshellcmd($bin) . ' --json',
            $descriptors,
            $pipes,
        );

        if (!is_resource($process)) {
            throw new RuntimeException("Failed to start runtime-autopilot binary: {$bin}");
        }

        fclose($pipes[0]);
        $stdout = stream_get_contents($pipes[1]);
        $stderr = stream_get_contents($pipes[2]);
        fclose($pipes[1]);
        fclose($pipes[2]);
        $exitCode = proc_close($process);

        if ($exitCode !== 0) {
            throw new RuntimeException(
                "runtime-autopilot exited with code {$exitCode}: " . trim((string) $stderr)
            );
        }

        if ($stdout === false || $stdout === '') {
            throw new RuntimeException('runtime-autopilot produced no output');
        }

        return (string) $stdout;
    }
}
