<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    // ADR-0010: capacity adjustments carry an explicit type and power a vacation
    // ledger. `type` (not sign) distinguishes the kinds; `status` splits penciled
    // (planned) from entered (confirmed); hours becomes decimal (half-days, a
    // 1.5h early finish). A per-year accrual is the one new stored input.
    public function up(): void
    {
        Schema::table('capacity_adjustments', function (Blueprint $table) {
            $table->string('type')->default('off')->after('date');
            $table->string('status')->default('planned')->after('hours');
        });

        // Backfill type from the old sign convention (ADR-0008). Existing rows
        // predate planned/confirmed and already moved capacity → confirmed.
        DB::table('capacity_adjustments')->where('hours', '>', 0)->update(['type' => 'extra']);
        DB::table('capacity_adjustments')->where('hours', '<', 0)->update(['type' => 'off']);
        DB::table('capacity_adjustments')->update(['status' => 'confirmed']);

        Schema::table('capacity_adjustments', function (Blueprint $table) {
            $table->decimal('hours', 6, 2)->change();
        });

        Schema::create('vacation_accruals', function (Blueprint $table) {
            $table->id();
            $table->integer('year')->unique();
            $table->decimal('hours', 6, 2);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('vacation_accruals');
        Schema::table('capacity_adjustments', function (Blueprint $table) {
            $table->integer('hours')->change();
            $table->dropColumn(['type', 'status']);
        });
    }
};
