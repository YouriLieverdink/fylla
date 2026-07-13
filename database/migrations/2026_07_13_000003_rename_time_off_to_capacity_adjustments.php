<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    // ADR-0008: one signed capacity_adjustments table replaces time_off.
    // hours is now signed (negative = time off, positive = extra day); one
    // row per date (unique). Base ± Σ adjustments = a week's capacity.
    public function up(): void
    {
        Schema::rename('time_off', 'capacity_adjustments');
        Schema::table('capacity_adjustments', function (Blueprint $table) {
            $table->integer('hours')->change();
            $table->unique('date');
        });
    }

    public function down(): void
    {
        Schema::table('capacity_adjustments', function (Blueprint $table) {
            $table->dropUnique(['date']);
            $table->unsignedInteger('hours')->change();
        });
        Schema::rename('capacity_adjustments', 'time_off');
    }
};
