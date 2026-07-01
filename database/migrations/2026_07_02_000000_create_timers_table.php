<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('timers', function (Blueprint $table) {
            $table->id();
            $table->foreignId('issue_id')->constrained();
            $table->timestamp('stopped_at')->nullable();
        });

        // One live timer per issue (Q7). Partial unique — buried/stopped timers don't count.
        DB::statement('CREATE UNIQUE INDEX timers_issue_live_unique ON timers (issue_id) WHERE stopped_at IS NULL');
    }

    public function down(): void
    {
        Schema::dropIfExists('timers');
    }
};
