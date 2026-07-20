<?php

namespace Tests\Feature;

use App\Jobs\SyncKendoUsers;
use App\Models\Developer;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class SyncKendoUsersTest extends TestCase
{
    use RefreshDatabase;

    private function user(int $id, array $overrides = []): array
    {
        return array_merge([
            'id' => $id,
            'first_name' => 'Sofia',
            'last_name' => 'Reyes',
            'email' => 'sofia@example.com',
            'deleted_at' => null,
            'profile_picture' => ['webp' => "https://cdn/$id.webp"],
        ], $overrides);
    }

    public function test_maps_name_active_and_avatar(): void
    {
        Http::fake(['*/users' => Http::response([
            $this->user(4),
            $this->user(5, ['first_name' => 'Gone', 'last_name' => 'Away', 'deleted_at' => '2026-01-01T00:00:00+00:00', 'profile_picture' => null]),
        ])]);

        SyncKendoUsers::dispatchSync();

        $active = Developer::firstWhere('kendo_id', 4);
        $this->assertSame('Sofia Reyes', $active->name);
        $this->assertTrue($active->active);
        $this->assertSame('https://cdn/4.webp', $active->avatar_url);

        $deleted = Developer::firstWhere('kendo_id', 5);
        $this->assertSame('Gone Away', $deleted->name);
        $this->assertFalse($deleted->active);
        $this->assertNull($deleted->avatar_url);
    }

    public function test_a_user_without_an_email_still_syncs(): void
    {
        Http::fake(['*/users' => Http::response([$this->user(9, ['email' => null])])]);

        SyncKendoUsers::dispatchSync();

        $this->assertNull(Developer::firstWhere('kendo_id', 9)->email);
    }

    public function test_is_idempotent_on_kendo_id(): void
    {
        Http::fake(['*/users' => Http::sequence()
            ->push([$this->user(4)])
            ->push([$this->user(4, ['first_name' => 'Renamed'])])]);

        SyncKendoUsers::dispatchSync();
        SyncKendoUsers::dispatchSync();

        $this->assertSame(1, Developer::count());
        $this->assertSame('Renamed Reyes', Developer::firstWhere('kendo_id', 4)->name);
    }
}
