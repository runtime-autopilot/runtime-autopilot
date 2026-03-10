<?php

declare(strict_types=1);

namespace RuntimeAutopilot;

use Illuminate\Support\ServiceProvider;
use Illuminate\Support\Facades\Log;

/**
 * Respects two escape hatches:
 *   AUTOPILOT_DISABLE=true  - skips all detection and configuration
 *   AUTOPILOT_DRY_RUN=true  - logs decisions but does not mutate config
 */
final class AutopilotServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        if ($this->isDisabled()) {
            return;
        }

        $this->app->singleton(RuntimeProfile::class, static function (): ?RuntimeProfile {
            return Probe::detect();
        });
    }

    public function boot(): void
    {
        if ($this->isDisabled()) {
            return;
        }

        $profile = $this->app->make(RuntimeProfile::class);
        if ($profile === null) {
            return;
        }

        $dryRun = $this->isDryRun();

        $this->applyLoggingDefaults($profile, $dryRun);
        $this->applyCacheDefaults($profile, $dryRun);
        $this->applyQueueDefaults($profile, $dryRun);
    }

    private function applyLoggingDefaults(RuntimeProfile $profile, bool $dryRun): void
    {
        if (!$profile->isReadOnly()) {
            return;
        }

        $this->decide(
            'root filesystem is read-only: switching logging.default to stderr',
            $dryRun,
            static function (): void {
                config(['logging.default' => 'stderr']);
            },
        );
    }

    private function applyCacheDefaults(RuntimeProfile $profile, bool $dryRun): void
    {
        if (!$profile->isReadOnly()) {
            return;
        }

        // Prefer Redis when the PHP `redis` extension is available.
        if (!extension_loaded('redis') && !class_exists(\Predis\Client::class)) {
            return;
        }

        $this->decide(
            'root filesystem is read-only and redis is available: switching cache.default to redis',
            $dryRun,
            static function (): void {
                config(['cache.default' => 'redis']);
            },
        );
    }

    private function applyQueueDefaults(RuntimeProfile $profile, bool $dryRun): void
    {
        if ($profile->role !== 'queue') {
            return;
        }

        $memMb = $profile->memMb();
        if ($memMb === null || $memMb >= 256) {
            return;
        }

        // Compute a conservative worker count: 1 worker per 64 MiB, minimum 1.
        $workers = max(1, (int) floor($memMb / 64));

        $this->decide(
            "queue role with {$memMb} MiB RAM: setting horizon supervisor maxProcesses to {$workers}",
            $dryRun,
            static function () use ($workers): void {
                config([
                    'horizon.environments.production.supervisor-default.maxProcesses' => $workers,
                ]);
            },
        );
    }

    /**
     * Log the decision message, then execute $action unless in dry-run mode.
     */
    private function decide(string $message, bool $dryRun, callable $action): void
    {
        Log::info('[runtime-autopilot] ' . $message . ($dryRun ? ' (dry-run, skipping)' : ''));
        if (!$dryRun) {
            $action();
        }
    }

    private function isDisabled(): bool
    {
        return filter_var(env('AUTOPILOT_DISABLE', false), FILTER_VALIDATE_BOOLEAN);
    }

    private function isDryRun(): bool
    {
        return filter_var(env('AUTOPILOT_DRY_RUN', false), FILTER_VALIDATE_BOOLEAN);
    }
}
