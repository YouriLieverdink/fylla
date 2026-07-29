<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('pull_requests', function (Blueprint $table) {
            // Current membership of the complete GitHub action-query union.
            // Retained timer-history rows remain mirrors, but leave the Worklist.
            $table->boolean('actionable')->default(true)->index();
        });

        // Settings override file defaults. Upgrade the former assignee feed so
        // an existing installation receives the new authored-PR lifecycle too.
        $setting = DB::table('settings')->where('key', 'github_pr_queries')->first();
        $queries = $setting ? json_decode($setting->value, true) : null;
        if (is_array($queries)) {
            $queries = array_values(array_unique(array_map(
                fn (string $query) => str_replace(
                    'assignee:@me',
                    'author:@me review:changes_requested',
                    $query,
                ),
                $queries,
            )));
            DB::table('settings')->where('key', 'github_pr_queries')->update([
                'value' => json_encode($queries, JSON_UNESCAPED_SLASHES),
            ]);
        }
    }

    public function down(): void
    {
        Schema::table('pull_requests', function (Blueprint $table) {
            $table->dropColumn('actionable');
        });
    }
};
