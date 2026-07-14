<?php

use App\Models\Issue;
use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        DB::statement('DROP INDEX IF EXISTS timers_issue_live_unique');

        Schema::table('timers', function (Blueprint $table) {
            if (! Schema::hasColumn('timers', 'timeable_type')) {
                $table->string('timeable_type')->nullable();
            }
            if (! Schema::hasColumn('timers', 'timeable_id')) {
                $table->unsignedBigInteger('timeable_id')->nullable();
            }
        });

        // Backfill existing timers as Issue subjects.
        DB::table('timers')->update(['timeable_type' => Issue::class]);
        DB::statement('UPDATE timers SET timeable_id = issue_id');

        Schema::table('timers', function (Blueprint $table) {
            $table->dropConstrainedForeignId('issue_id');
        });

        // One live timer per subject (ADR-0005/0009). Partial — stopped timers don't count.
        DB::statement('CREATE UNIQUE INDEX timers_timeable_live_unique ON timers (timeable_type, timeable_id) WHERE stopped_at IS NULL');
    }

    public function down(): void
    {
        DB::statement('DROP INDEX IF EXISTS timers_timeable_live_unique');

        Schema::table('timers', function (Blueprint $table) {
            $table->foreignId('issue_id')->nullable()->constrained();
        });

        DB::statement('UPDATE timers SET issue_id = timeable_id');

        Schema::table('timers', function (Blueprint $table) {
            $table->dropColumn(['timeable_type', 'timeable_id']);
        });
    }
};
