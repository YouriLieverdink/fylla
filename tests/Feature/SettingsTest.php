<?php

namespace Tests\Feature;

use App\Models\Setting;
use App\Providers\SettingsProvider;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class SettingsTest extends TestCase
{
    use RefreshDatabase;

    public function test_edit_renders_the_settings_page_with_effective_values(): void
    {
        $this->get('/settings')
            ->assertOk()
            ->assertInertia(fn ($page) => $page
                ->component('Settings')
                ->where('values.utilization_target', 75)
                ->where('values.display_timezone', 'Europe/Amsterdam')
                ->has('values.github_pr_queries'));
    }

    public function test_update_persists_an_override_row(): void
    {
        $this->put('/settings', $this->payload(['utilization_target' => 80]))
            ->assertRedirect('/settings');

        $this->assertSame(80, Setting::where('key', 'utilization_target')->value('value'));
    }

    /** The override seam: on boot the provider overrides the file default.
     * (In production each request is a fresh boot; here we boot it directly.) */
    public function test_provider_overrides_config_from_the_settings_table(): void
    {
        Setting::create(['key' => 'utilization_target', 'value' => 80]);

        $this->assertSame(75, config('fylla.utilization_target')); // file default
        (new SettingsProvider($this->app))->boot();
        $this->assertSame(80, config('fylla.utilization_target')); // overridden
    }

    public function test_list_fields_round_trip_as_arrays(): void
    {
        $this->put('/settings', $this->payload([
            'github_pr_queries' => ['org:Foo author:@me', 'org:Bar assignee:@me'],
        ]))->assertRedirect('/settings');

        $this->assertSame(
            ['org:Foo author:@me', 'org:Bar assignee:@me'],
            Setting::where('key', 'github_pr_queries')->value('value'),
        );
    }

    public function test_a_value_at_the_file_default_writes_no_row_and_resets_an_override(): void
    {
        Setting::create(['key' => 'worklog_sync_days', 'value' => 30]);

        // payload() carries the file defaults for these two keys.
        $this->put('/settings', $this->payload())->assertRedirect('/settings');

        $this->assertDatabaseMissing('settings', ['key' => 'worklog_sync_days']); // reset
        $this->assertDatabaseMissing('settings', ['key' => 'utilization_target']); // default → no row
    }

    public function test_soft_floor_above_target_is_rejected(): void
    {
        $this->put('/settings', $this->payload([
            'utilization_target' => 70,
            'utilization_soft_floor' => 80,
        ]))->assertSessionHasErrors('utilization_soft_floor');

        $this->assertDatabaseCount('settings', 0);
    }

    public function test_out_of_range_value_is_rejected(): void
    {
        $this->put('/settings', $this->payload(['contracted_off_weekday' => 9]))
            ->assertSessionHasErrors('contracted_off_weekday');

        $this->assertDatabaseCount('settings', 0);
    }

    /** A full valid payload with the given overrides merged in. */
    private function payload(array $overrides = []): array
    {
        return array_merge([
            'kendo_user_id' => 'user-1',
            'worklog_sync_days' => 90,
            'display_timezone' => 'Europe/Amsterdam',
            'github_pr_queries' => ['org:Back-to-code assignee:@me'],
            'github_pr_exclude_repos' => ['Back-to-code/daymate-api'],
            'contracted_hours_per_week' => 32,
            'contracted_off_weekday' => 5,
            'utilization_window_weeks' => 13,
            'utilization_target' => 75,
            'utilization_soft_floor' => 73,
        ], $overrides);
    }
}
