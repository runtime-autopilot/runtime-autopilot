<?php

declare(strict_types=1);

namespace RuntimeAutopilot\Tests;

use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;
use RuntimeAutopilot\RuntimeProfile;
use RuntimeAutopilot\Probe;

/**
 * Tests for the Laravel adapter.
 * Probe::detect() is tested by overriding environment variables to point at
 * fixture JSON files via RUNTIME_AUTOPILOT_URL (file:// URIs are NOT supported
 * by PHP's file_get_contents for http contexts), so we test Probe by mocking
 * the JSON directly and testing RuntimeProfile::fromArray thoroughly.
 */
final class ProbeTest extends TestCase
{
    // --- RuntimeProfile::fromArray ---

    #[DataProvider('profileProvider')]
    public function testFromArray(array $data, ?int $memBytes, ?float $cpuEffective, bool $readOnly, string $platform, string $role): void
    {
        $profile = RuntimeProfile::fromArray($data);

        $this->assertSame($memBytes, $profile->memBytes);
        $this->assertEqualsWithDelta($cpuEffective ?? 0.0, $profile->cpuEffective ?? 0.0, 0.0001);
        if ($cpuEffective === null) {
            $this->assertNull($profile->cpuEffective);
        }
        $this->assertSame($readOnly, $profile->rootReadOnly);
        $this->assertSame($platform, $profile->platform);
        $this->assertSame($role, $profile->role);
    }

    public static function profileProvider(): array
    {
        return [
            'kubernetes web container' => [
                [
                    'mem_bytes'      => 268435456,
                    'cpu_effective'  => 2.0,
                    'root_read_only' => false,
                    'writable_paths' => ['/tmp'],
                    'platform'       => 'kubernetes',
                    'role'           => 'web',
                ],
                268435456, 2.0, false, 'kubernetes', 'web',
            ],
            'read-only bare-metal queue' => [
                [
                    'mem_bytes'      => null,
                    'cpu_effective'  => null,
                    'root_read_only' => true,
                    'writable_paths' => [],
                    'platform'       => 'bare-metal',
                    'role'           => 'queue',
                ],
                null, null, true, 'bare-metal', 'queue',
            ],
            'ecs scheduler' => [
                [
                    'mem_bytes'      => 536870912,
                    'cpu_effective'  => 1.5,
                    'root_read_only' => false,
                    'writable_paths' => ['/tmp', '/app'],
                    'platform'       => 'ecs',
                    'role'           => 'scheduler',
                ],
                536870912, 1.5, false, 'ecs', 'scheduler',
            ],
            'missing fields use defaults' => [
                [],
                null, null, false, 'bare-metal', 'cli',
            ],
        ];
    }

    // --- RuntimeProfile::sizeClass ---

    #[DataProvider('sizeClassProvider')]
    public function testSizeClass(?int $memBytes, string $expected): void
    {
        $profile = RuntimeProfile::fromArray(['mem_bytes' => $memBytes]);
        $this->assertSame($expected, $profile->sizeClass());
    }

    public static function sizeClassProvider(): array
    {
        return [
            'null => tiny'                     => [null, 'tiny'],
            '128 MiB => tiny'                  => [128 * 1024 * 1024, 'tiny'],
            '256 MiB => medium'                => [256 * 1024 * 1024, 'medium'],
            '512 MiB => medium'                => [512 * 1024 * 1024, 'medium'],
            '1024 MiB => large'                => [1024 * 1024 * 1024, 'large'],
            '2 GiB => large'                   => [2 * 1024 * 1024 * 1024, 'large'],
        ];
    }

    // --- RuntimeProfile::memMb ---

    public function testMemMbNull(): void
    {
        $profile = RuntimeProfile::fromArray([]);
        $this->assertNull($profile->memMb());
    }

    public function testMemMb512(): void
    {
        $profile = RuntimeProfile::fromArray(['mem_bytes' => 512 * 1024 * 1024]);
        $this->assertEqualsWithDelta(512.0, $profile->memMb(), 0.01);
    }

    // --- Probe JSON decoding ---

    public function testProbeFromFixtureJson(): void
    {
        $json = json_encode([
            'mem_bytes'      => 268435456,
            'cpu_effective'  => 2.0,
            'root_read_only' => false,
            'writable_paths' => ['/tmp'],
            'platform'       => 'kubernetes',
            'role'           => 'web',
        ]);

        // Write fixture to a temp file and override the binary lookup via a
        // custom env that points to a shell script echoing that JSON.
        $fixture = tempnam(sys_get_temp_dir(), 'autopilot-') . '.json';
        file_put_contents($fixture, $json);

        // Create a tiny shell script that acts as a fake binary.
        $bin = tempnam(sys_get_temp_dir(), 'autopilot-bin-');
        file_put_contents($bin, "#!/bin/sh\ncat " . escapeshellarg($fixture) . "\n");
        chmod($bin, 0755);

        putenv('RUNTIME_AUTOPILOT_BIN=' . $bin);
        putenv('RUNTIME_AUTOPILOT_URL=');

        $profile = Probe::detect();

        putenv('RUNTIME_AUTOPILOT_BIN=');
        unlink($fixture);
        unlink($bin);

        $this->assertNotNull($profile);
        $this->assertSame(268435456, $profile->memBytes);
        $this->assertSame('kubernetes', $profile->platform);
    }
}
