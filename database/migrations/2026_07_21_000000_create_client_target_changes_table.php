<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Effective-dated overrides of clients.monthly_target_hours (#66):
        // "from month M onward, the target is H", persisting forward until the
        // next change. The clients column stays as the baseline default.
        Schema::create('client_target_changes', function (Blueprint $table) {
            $table->id();
            $table->foreignId('client_id')->constrained()->cascadeOnDelete();
            $table->date('effective_from'); // always first-of-month
            $table->unsignedInteger('hours');
            $table->unique(['client_id', 'effective_from']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('client_target_changes');
    }
};
