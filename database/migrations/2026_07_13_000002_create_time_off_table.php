<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Fylla-native (ADR-0004): Kendo has no leave concept. Logged time off
        // shrinks that week's utilization capacity (denominator), never the
        // billable numerator. hours as int (8 = a full day).
        Schema::create('time_off', function (Blueprint $table) {
            $table->id();
            $table->date('date');
            $table->unsignedInteger('hours');
            $table->string('reason')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('time_off');
    }
};
