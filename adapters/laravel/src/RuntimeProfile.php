<?php

declare(strict_types=1);

namespace RuntimeAutopilot;

final class RuntimeProfile
{
    public function __construct(
        public readonly ?int $memBytes,
        public readonly ?float $cpuEffective,
        public readonly bool $rootReadOnly,
        public readonly array $writablePaths,
        public readonly string $platform,
        public readonly string $role,
    ) {}

    public static function fromArray(array $data): self
    {
        return new self(
            memBytes: isset($data['mem_bytes']) ? (int) $data['mem_bytes'] : null,
            cpuEffective: isset($data['cpu_effective']) ? (float) $data['cpu_effective'] : null,
            rootReadOnly: (bool) ($data['root_read_only'] ?? false),
            writablePaths: array_values(array_map('strval', $data['writable_paths'] ?? [])),
            platform: (string) ($data['platform'] ?? 'bare-metal'),
            role: (string) ($data['role'] ?? 'cli'),
        );
    }

    public function memMb(): ?float
    {
        return $this->memBytes !== null ? $this->memBytes / (1024 * 1024) : null;
    }


    public function sizeClass(): string
    {
        $mb = $this->memMb();
        if ($mb === null || $mb < 256) {
            return 'tiny';
        }
        if ($mb < 1024) {
            return 'medium';
        }
        return 'large';
    }

    public function isReadOnly(): bool
    {
        return $this->rootReadOnly;
    }

    public function cpuEffective(): ?float
    {
        return $this->cpuEffective;
    }
}
